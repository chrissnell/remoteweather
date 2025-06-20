package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

const version = "3.0-" + runtime.GOOS + "/" + runtime.GOARCH

var zapLogger *zap.Logger
var log *zap.SugaredLogger

var debug *bool

func main() {
	var wg sync.WaitGroup
	var err error

	cfgFile := flag.String("config", "config.yaml", "Path to config file (default: ./config.yaml)")
	debug = flag.Bool("debug", false, "Turn on debugging output")
	flag.Parse()

	// Set up our logger
	if *debug {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("can't initialize zap logger: %v\n", err)
		os.Exit(1)
	}
	defer zapLogger.Sync()
	log = zapLogger.Sugar()

	// Read our server configuration
	filename, _ := filepath.Abs(*cfgFile)
	cfg, err := NewConfig(filename)
	if err != nil {
		log.Errorf("error reading config file. Did you pass the -config flag? Run with -h for help: %v", err)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Initialize the storage manager
	distributor, err := NewStorageManager(ctx, &wg, &cfg)
	if err != nil {
		log.Errorf("failed to create storage manager: %v", err)
		cancel()
		os.Exit(1)
	}

	// Initialize the weather station manager
	wsm, err := NewWeatherStationManager(ctx, &wg, &cfg, distributor.ReadingDistributor, log)
	if err != nil {
		log.Errorf("could not create weather station manager: %v", err)
		cancel()
		os.Exit(1)
	}
	go wsm.StartWeatherStations()

	// Initialize the controller manager
	cm, err := NewControllerManager(ctx, &wg, &cfg, log)
	if err != nil {
		log.Errorf("could not create controller manager: %v", err)
		cancel()
		os.Exit(1)
	}
	err = cm.StartControllers()
	if err != nil {
		log.Errorf("could not start controllers: %v", err)
		cancel()
		os.Exit(1)
	}

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func(cancel context.CancelFunc) {
		// If we get a SIGINT or SIGTERM, cancel the context and unblock 'done'
		// to trigger a program shutdown
		<-sigs
		log.Info("shutdown signal received, initiating graceful shutdown...")
		cancel()
		close(done)
	}(cancel)

	// Wait for 'done' to unblock before terminating
	<-done

	log.Info("waiting for all workers to terminate...")
	// Also wait for all of our workers to terminate before terminating the program
	wg.Wait()
	log.Info("shutdown complete")

}
