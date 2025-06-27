package app

import (
	"context"
	htmltemplate "html/template"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/managers"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// App represents the main application
type App struct {
	config *types.Config
	logger *zap.SugaredLogger
}

// New creates a new application instance
func New(cfgData *config.ConfigData, logger *zap.SugaredLogger) *App {
	cfg := convertToLegacyConfig(cfgData)
	return &App{
		config: &cfg,
		logger: logger,
	}
}

// Run starts the application and blocks until shutdown
func (a *App) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize the storage manager
	storageManager, err := managers.NewStorageManager(ctx, &wg, a.config)
	if err != nil {
		return err
	}

	// Initialize the weather station manager
	wsm, err := managers.NewWeatherStationManager(ctx, &wg, a.config, storageManager.ReadingDistributor, a.logger)
	if err != nil {
		return err
	}
	go wsm.StartWeatherStations()

	// Initialize the controller manager
	cm, err := managers.NewControllerManager(ctx, &wg, a.config, a.logger)
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

// convertToLegacyConfig converts from the new config.ConfigData structure
// to the legacy Config struct that the rest of the application expects.
func convertToLegacyConfig(cfgData *config.ConfigData) types.Config {
	cfg := types.Config{
		Devices:     make([]types.DeviceConfig, len(cfgData.Devices)),
		Controllers: make([]types.ControllerConfig, len(cfgData.Controllers)),
	}

	// Convert devices
	for i, device := range cfgData.Devices {
		cfg.Devices[i] = types.DeviceConfig{
			Name:              device.Name,
			Type:              device.Type,
			Hostname:          device.Hostname,
			Port:              device.Port,
			SerialDevice:      device.SerialDevice,
			Baud:              device.Baud,
			WindDirCorrection: device.WindDirCorrection,
			BaseSnowDistance:  device.BaseSnowDistance,
			Solar: types.SolarConfig{
				Latitude:  device.Solar.Latitude,
				Longitude: device.Solar.Longitude,
				Altitude:  device.Solar.Altitude,
			},
		}
	}

	// Convert storage configuration
	cfg.Storage = types.StorageConfig{}

	if cfgData.Storage.InfluxDB != nil {
		cfg.Storage.InfluxDB = types.InfluxDBConfig{
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
		cfg.Storage.TimescaleDB = types.TimescaleDBConfig{
			ConnectionString: cfgData.Storage.TimescaleDB.ConnectionString,
		}
	}

	if cfgData.Storage.GRPC != nil {
		cfg.Storage.GRPC = types.GRPCConfig{
			Cert:           cfgData.Storage.GRPC.Cert,
			Key:            cfgData.Storage.GRPC.Key,
			ListenAddr:     cfgData.Storage.GRPC.ListenAddr,
			Port:           cfgData.Storage.GRPC.Port,
			PullFromDevice: cfgData.Storage.GRPC.PullFromDevice,
		}
	}

	if cfgData.Storage.APRS != nil {
		cfg.Storage.APRS = types.APRSConfig{
			Callsign:     cfgData.Storage.APRS.Callsign,
			Passcode:     cfgData.Storage.APRS.Passcode,
			APRSISServer: cfgData.Storage.APRS.APRSISServer,
			Location: types.Point{
				Lat: cfgData.Storage.APRS.Location.Lat,
				Lon: cfgData.Storage.APRS.Location.Lon,
			},
		}
	}

	// Convert controllers
	for i, controller := range cfgData.Controllers {
		cfg.Controllers[i] = types.ControllerConfig{
			Type: controller.Type,
		}

		if controller.PWSWeather != nil {
			cfg.Controllers[i].PWSWeather = types.PWSWeatherConfig{
				StationID:      controller.PWSWeather.StationID,
				APIKey:         controller.PWSWeather.APIKey,
				APIEndpoint:    controller.PWSWeather.APIEndpoint,
				UploadInterval: controller.PWSWeather.UploadInterval,
				PullFromDevice: controller.PWSWeather.PullFromDevice,
			}
		}

		if controller.WeatherUnderground != nil {
			cfg.Controllers[i].WeatherUnderground = types.WeatherUndergroundConfig{
				StationID:      controller.WeatherUnderground.StationID,
				APIKey:         controller.WeatherUnderground.APIKey,
				UploadInterval: controller.WeatherUnderground.UploadInterval,
				PullFromDevice: controller.WeatherUnderground.PullFromDevice,
				APIEndpoint:    controller.WeatherUnderground.APIEndpoint,
			}
		}

		if controller.AerisWeather != nil {
			cfg.Controllers[i].AerisWeather = types.AerisWeatherConfig{
				APIClientID:     controller.AerisWeather.APIClientID,
				APIClientSecret: controller.AerisWeather.APIClientSecret,
				APIEndpoint:     controller.AerisWeather.APIEndpoint,
				Location:        controller.AerisWeather.Location,
			}
		}

		if controller.RESTServer != nil {
			cfg.Controllers[i].RESTServer = types.RESTServerConfig{
				Cert:       controller.RESTServer.Cert,
				Key:        controller.RESTServer.Key,
				Port:       controller.RESTServer.Port,
				ListenAddr: controller.RESTServer.ListenAddr,
				WeatherSiteConfig: types.WeatherSiteConfig{
					StationName:      controller.RESTServer.WeatherSiteConfig.StationName,
					PullFromDevice:   controller.RESTServer.WeatherSiteConfig.PullFromDevice,
					SnowEnabled:      controller.RESTServer.WeatherSiteConfig.SnowEnabled,
					SnowDevice:       controller.RESTServer.WeatherSiteConfig.SnowDevice,
					PageTitle:        controller.RESTServer.WeatherSiteConfig.PageTitle,
					AboutStationHTML: htmltemplate.HTML(controller.RESTServer.WeatherSiteConfig.AboutStationHTML),
				},
			}
		}

		if controller.ManagementAPI != nil {
			cfg.Controllers[i].ManagementAPI = types.ManagementAPIConfig{
				Cert:       controller.ManagementAPI.Cert,
				Key:        controller.ManagementAPI.Key,
				Port:       controller.ManagementAPI.Port,
				ListenAddr: controller.ManagementAPI.ListenAddr,
				AuthToken:  controller.ManagementAPI.AuthToken,
				EnableCORS: controller.ManagementAPI.EnableCORS,
			}
		}
	}

	return cfg
}
