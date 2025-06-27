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
	configProvider    config.ConfigProvider
	logger            *zap.SugaredLogger
	storageManager    *managers.StorageManager
	weatherManager    managers.WeatherStationManager
	controllerManager managers.ControllerManager
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
	var err error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize the storage manager
	a.storageManager, err = managers.NewStorageManager(ctx, &wg, a.configProvider)
	if err != nil {
		return err
	}

	// Initialize the weather station manager
	a.weatherManager, err = managers.NewWeatherStationManager(ctx, &wg, a.configProvider, a.storageManager.ReadingDistributor, a.logger)
	if err != nil {
		return err
	}
	go a.weatherManager.StartWeatherStations()

	// Initialize the controller manager
	a.controllerManager, err = managers.NewControllerManager(ctx, &wg, a.configProvider, a.logger, a)
	if err != nil {
		return err
	}
	err = a.controllerManager.StartControllers()
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

// ReloadConfiguration reloads configuration across all managers
func (a *App) ReloadConfiguration(ctx context.Context) error {
	a.logger.Info("Reloading configuration across all managers...")

	var wg sync.WaitGroup

	// Reload storage configuration
	if err := a.storageManager.ReloadStorageConfig(ctx, &wg, a.configProvider); err != nil {
		a.logger.Errorf("Failed to reload storage configuration: %v", err)
		return err
	}

	// Reload weather station configuration
	if err := a.weatherManager.ReloadWeatherStationsConfig(); err != nil {
		a.logger.Errorf("Failed to reload weather station configuration: %v", err)
		return err
	}

	// Reload controller configuration
	if err := a.controllerManager.ReloadControllersConfig(); err != nil {
		a.logger.Errorf("Failed to reload controller configuration: %v", err)
		return err
	}

	a.logger.Info("Configuration reloaded successfully")
	return nil
}
