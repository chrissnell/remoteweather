package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/chrissnell/remoteweather/internal/app"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

const version = "5.0-" + runtime.GOOS + "/" + runtime.GOARCH

func main() {
	cfgFile := flag.String("config", "config.yaml", "Path to configuration source:\n\t\t\t  YAML: config.yaml, weather-station.yaml\n\t\t\t  SQLite: config.db, weather-station.db\n\t\t\t  Use 'config-convert' tool to convert YAML→SQLite")
	cfgBackend := flag.String("config-backend", "yaml", "Configuration backend type: 'yaml' for YAML files, 'sqlite' for SQLite databases")
	debug := flag.Bool("debug", false, "Turn on debugging output")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("remoteweather %s\n", version)
		os.Exit(0)
	}

	// Set up logging
	if err := log.Init(*debug); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Load configuration
	cfgData, err := loadConfig(*cfgFile, *cfgBackend)
	if err != nil {
		log.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Create and run the application
	application := app.New(cfgData, log.GetSugaredLogger())
	if err := application.Run(context.Background()); err != nil {
		log.Errorf("Application error: %v", err)
		os.Exit(1)
	}
}

func loadConfig(cfgFile, cfgBackend string) (*config.ConfigData, error) {
	filename, _ := filepath.Abs(cfgFile)

	var provider config.ConfigProvider
	var err error

	switch cfgBackend {
	case "yaml":
		provider = config.NewYAMLProvider(filename)
	case "sqlite":
		provider, err = config.NewSQLiteProvider(filename)
		if err != nil {
			return nil, fmt.Errorf("error creating SQLite provider: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported configuration backend: %s. Use 'yaml' or 'sqlite'", cfgBackend)
	}

	cfgData, err := provider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file. Did you pass the -config flag? Run with -h for help: %w", err)
	}

	return cfgData, nil
}
