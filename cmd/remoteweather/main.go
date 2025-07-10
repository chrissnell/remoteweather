// Package main provides the main remoteweather application for collecting and serving weather data.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chrissnell/remoteweather/internal/app"
	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/google/uuid"
)

func main() {
	cfgFile := flag.String("config", "config.db", "Path to SQLite configuration database")
	debug := flag.Bool("debug", false, "Turn on debugging output")
	showVersion := flag.Bool("version", false, "Show version and exit")
	enableManagementAPI := flag.Bool("enable-management-api", false, "Enable management API with default configuration even if not in config")
	flag.Parse()

	if *showVersion {
		fmt.Printf("remoteweather %s (%s/%s)\n", constants.Version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// Set up logging
	if err := log.Init(*debug); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Create and run the application
	configProvider, err := createConfigProvider(*cfgFile)
	if err != nil {
		log.Errorf("Failed to create config provider: %v", err)
		os.Exit(1)
	}
	defer configProvider.Close()

	application := app.New(configProvider, log.GetSugaredLogger())
	if err := application.Run(context.Background(), *enableManagementAPI); err != nil {
		log.Errorf("Application error: %v", err)
		os.Exit(1)
	}
}

func createConfigProvider(cfgFile string) (config.ConfigProvider, error) {
	filename, _ := filepath.Abs(cfgFile)

	// Check if database file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Infof("Configuration database does not exist. Creating bootstrap database at: %s", filename)
		if err := createBootstrapDatabase(filename); err != nil {
			return nil, fmt.Errorf("failed to create bootstrap database: %w", err)
		}
		log.Infof("Bootstrap database created successfully!")
		log.Infof("You can now configure your weather stations and websites using the management API at http://localhost:8081")
	}

	provider, err := config.NewSQLiteProvider(filename)
	if err != nil {
		return nil, fmt.Errorf("error creating SQLite provider: %w", err)
	}

	// Test that we can load the config
	_, err = provider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config database: %w", err)
	}

	// Wrap with caching layer for performance (30 second cache)
	cachedProvider := config.NewCachedProvider(provider, 30*time.Second)

	return cachedProvider, nil
}

func createBootstrapDatabase(dbPath string) error {
	// Create the database with basic structure
	provider, err := config.NewSQLiteProvider(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer provider.Close()

	// Generate UUID token (with hyphens for readability)
	token := uuid.New().String()

	// Bootstrap with minimal management API configuration
	managementController := &config.ControllerData{
		Type: "management",
		ManagementAPI: &config.ManagementAPIData{
			Port:       8081,
			ListenAddr: "localhost",
			AuthToken:  token,
		},
	}

	err = provider.AddController(managementController)
	if err != nil {
		return fmt.Errorf("failed to add management API controller: %w", err)
	}

	log.Info("═══════════════════════════════════════════════════════════════")
	log.Info("         BOOTSTRAP: MANAGEMENT API ACCESS TOKEN               ")
	log.Info("═══════════════════════════════════════════════════════════════")
	log.Infof("   Token: %s", token)
	log.Info("   *** SAVE THIS TOKEN - YOU'LL NEED IT TO ACCESS THE API ***")
	log.Info("   Management API will be available at http://localhost:8081")
	log.Info("═══════════════════════════════════════════════════════════════")

	return nil
}
