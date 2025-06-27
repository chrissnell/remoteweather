package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/managers"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// App represents the main application
type App struct {
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
}

// New creates a new application instance
func New(configProvider config.ConfigProvider, logger *zap.SugaredLogger) *App {
	return &App{
		configProvider: configProvider,
		logger:         logger,
	}
}

// Run starts the application and blocks until shutdown
func (a *App) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize the storage manager
	storageManager, err := managers.NewStorageManager(ctx, &wg, a.configProvider)
	if err != nil {
		return err
	}

	// Initialize the weather station manager
	wsm, err := managers.NewWeatherStationManager(ctx, &wg, a.configProvider, storageManager.ReadingDistributor, a.logger)
	if err != nil {
		return err
	}
	go wsm.StartWeatherStations()

	// Initialize the controller manager
	cm, err := managers.NewControllerManager(ctx, &wg, a.configProvider, a.logger)
	if err != nil {
		return err
	}
	err = cm.StartControllers()
	if err != nil {
		return err
	}

	log.Info("Application started successfully")

	// Set up signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case <-sigs:
		log.Info("shutdown signal received, initiating graceful shutdown...")
	case <-ctx.Done():
		log.Info("context cancelled, shutting down...")
	}

	// Cancel context to signal all goroutines to stop
	cancel()

	// Wait for all workers to terminate
	log.Info("waiting for all workers to terminate...")
	wg.Wait()
	log.Info("shutdown complete")

	return nil
}
