package main

import (
	"context"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

func main() {
	var wg sync.WaitGroup
	var err error

	cfgFile := flag.String("config", "config.yaml", "Path to configuration source:\n\t\t\t  YAML: config.yaml, weather-station.yaml\n\t\t\t  SQLite: config.db, weather-station.db\n\t\t\t  Use 'config-convert' tool to convert YAML→SQLite")
	cfgBackend := flag.String("config-backend", "yaml", "Configuration backend type: 'yaml' for YAML files, 'sqlite' for SQLite databases")
	debug := flag.Bool("debug", false, "Turn on debugging output")
	flag.Parse()

	// Set up our logger
	err = log.Init(*debug)
	if err != nil {
		fmt.Printf("can't initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Read our server configuration using the specified backend
	filename, _ := filepath.Abs(*cfgFile)

	var provider config.ConfigProvider
	switch *cfgBackend {
	case "yaml":
		provider = config.NewYAMLProvider(filename)
	case "sqlite":
		provider, err = config.NewSQLiteProvider(filename)
		if err != nil {
			log.Errorf("error creating SQLite provider: %v", err)
			os.Exit(1)
		}
	default:
		log.Errorf("unsupported configuration backend: %s. Use 'yaml' or 'sqlite'", *cfgBackend)
		os.Exit(1)
	}

	cfgData, err := provider.LoadConfig()
	if err != nil {
		log.Errorf("error reading config file. Did you pass the -config flag? Run with -h for help: %v", err)
		os.Exit(1)
	}

	// Convert to legacy Config struct for now
	cfg := convertToLegacyConfig(cfgData)

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
	wsm, err := NewWeatherStationManager(ctx, &wg, &cfg, distributor.ReadingDistributor, nil)
	if err != nil {
		log.Errorf("could not create weather station manager: %v", err)
		cancel()
		os.Exit(1)
	}
	go wsm.StartWeatherStations()

	// Initialize the controller manager
	cm, err := NewControllerManager(ctx, &wg, &cfg, nil)
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

// convertToLegacyConfig converts from the new config.ConfigData structure
// to the legacy Config struct that the rest of the application expects.
// This allows us to gradually migrate the application to use the new config system.
func convertToLegacyConfig(cfgData *config.ConfigData) Config {
	cfg := Config{
		Devices:     make([]DeviceConfig, len(cfgData.Devices)),
		Controllers: make([]ControllerConfig, len(cfgData.Controllers)),
	}

	// Convert devices
	for i, device := range cfgData.Devices {
		cfg.Devices[i] = DeviceConfig{
			Name:              device.Name,
			Type:              device.Type,
			Hostname:          device.Hostname,
			Port:              device.Port,
			SerialDevice:      device.SerialDevice,
			Baud:              device.Baud,
			WindDirCorrection: device.WindDirCorrection,
			BaseSnowDistance:  device.BaseSnowDistance,
			Solar: SolarConfig{
				Latitude:  device.Solar.Latitude,
				Longitude: device.Solar.Longitude,
				Altitude:  device.Solar.Altitude,
			},
		}
	}

	// Convert storage configuration
	cfg.Storage = StorageConfig{}

	if cfgData.Storage.InfluxDB != nil {
		cfg.Storage.InfluxDB = InfluxDBConfig{
			Scheme:   cfgData.Storage.InfluxDB.Scheme,
			Host:     cfgData.Storage.InfluxDB.Host,
			Port:     cfgData.Storage.InfluxDB.Port,
			Username: cfgData.Storage.InfluxDB.Username,
			Password: cfgData.Storage.InfluxDB.Password,
			Database: cfgData.Storage.InfluxDB.Database,
			Protocol: cfgData.Storage.InfluxDB.Protocol,
		}
	}

	if cfgData.Storage.TimescaleDB != nil {
		cfg.Storage.TimescaleDB = TimescaleDBConfig{
			ConnectionString: cfgData.Storage.TimescaleDB.ConnectionString,
		}
	}

	if cfgData.Storage.GRPC != nil {
		cfg.Storage.GRPC = GRPCConfig{
			Cert:           cfgData.Storage.GRPC.Cert,
			Key:            cfgData.Storage.GRPC.Key,
			ListenAddr:     cfgData.Storage.GRPC.ListenAddr,
			Port:           cfgData.Storage.GRPC.Port,
			PullFromDevice: cfgData.Storage.GRPC.PullFromDevice,
		}
	}

	if cfgData.Storage.APRS != nil {
		cfg.Storage.APRS = APRSConfig{
			Callsign:     cfgData.Storage.APRS.Callsign,
			Passcode:     cfgData.Storage.APRS.Passcode,
			APRSISServer: cfgData.Storage.APRS.APRSISServer,
			Location: Point{
				Lat: cfgData.Storage.APRS.Location.Lat,
				Lon: cfgData.Storage.APRS.Location.Lon,
			},
		}
	}

	// Convert controllers
	for i, controller := range cfgData.Controllers {
		cfg.Controllers[i] = ControllerConfig{
			Type: controller.Type,
		}

		if controller.PWSWeather != nil {
			cfg.Controllers[i].PWSWeather = PWSWeatherConfig{
				StationID:      controller.PWSWeather.StationID,
				APIKey:         controller.PWSWeather.APIKey,
				APIEndpoint:    controller.PWSWeather.APIEndpoint,
				UploadInterval: controller.PWSWeather.UploadInterval,
				PullFromDevice: controller.PWSWeather.PullFromDevice,
			}
		}

		if controller.WeatherUnderground != nil {
			cfg.Controllers[i].WeatherUnderground = WeatherUndergroundConfig{
				StationID:      controller.WeatherUnderground.StationID,
				APIKey:         controller.WeatherUnderground.APIKey,
				UploadInterval: controller.WeatherUnderground.UploadInterval,
				PullFromDevice: controller.WeatherUnderground.PullFromDevice,
				APIEndpoint:    controller.WeatherUnderground.APIEndpoint,
			}
		}

		if controller.AerisWeather != nil {
			cfg.Controllers[i].AerisWeather = AerisWeatherConfig{
				APIClientID:     controller.AerisWeather.APIClientID,
				APIClientSecret: controller.AerisWeather.APIClientSecret,
				APIEndpoint:     controller.AerisWeather.APIEndpoint,
				Location:        controller.AerisWeather.Location,
			}
		}

		if controller.RESTServer != nil {
			cfg.Controllers[i].RESTServer = RESTServerConfig{
				Cert:       controller.RESTServer.Cert,
				Key:        controller.RESTServer.Key,
				Port:       controller.RESTServer.Port,
				ListenAddr: controller.RESTServer.ListenAddr,
				WeatherSiteConfig: WeatherSiteConfig{
					StationName:      controller.RESTServer.WeatherSiteConfig.StationName,
					PullFromDevice:   controller.RESTServer.WeatherSiteConfig.PullFromDevice,
					SnowEnabled:      controller.RESTServer.WeatherSiteConfig.SnowEnabled,
					SnowDevice:       controller.RESTServer.WeatherSiteConfig.SnowDevice,
					PageTitle:        controller.RESTServer.WeatherSiteConfig.PageTitle,
					AboutStationHTML: htmltemplate.HTML(controller.RESTServer.WeatherSiteConfig.AboutStationHTML),
				},
			}
		}
	}

	return cfg
}
