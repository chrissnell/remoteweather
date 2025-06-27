package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/internal/weatherstations/campbell"
	"github.com/chrissnell/remoteweather/internal/weatherstations/davis"
	"github.com/chrissnell/remoteweather/internal/weatherstations/snowgauge"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// WeatherStationManager interface for the weather station manager
type WeatherStationManager interface {
	StartWeatherStations() error
}

// NewWeatherStationManager creates a WeatherStationManager object, populated with all configured weather stations
func NewWeatherStationManager(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, distributor chan types.Reading, logger *zap.SugaredLogger) (WeatherStationManager, error) {
	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	wsm := &weatherStationManager{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		distributor:    distributor,
		logger:         logger,
	}

	// Create weather stations directly from config data
	for _, deviceConfig := range cfgData.Devices {
		station, err := createStationFromConfig(ctx, wg, configProvider, deviceConfig.Name, distributor, logger)
		if err != nil {
			return nil, fmt.Errorf("error creating weather station [%s]: %w", deviceConfig.Name, err)
		}
		wsm.stations = append(wsm.stations, station)
	}

	return wsm, nil
}

type weatherStationManager struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	distributor    chan types.Reading
	logger         *zap.SugaredLogger
	stations       []weatherstations.WeatherStation
}

func (w *weatherStationManager) StartWeatherStations() error {
	w.logger.Info("Weather station manager started (clean orchestration)")
	for _, station := range w.stations {
		w.logger.Infof("Starting weather station [%v]...", station.StationName())
		if err := station.StartWeatherStation(); err != nil {
			return fmt.Errorf("failed to start weather station [%s]: %w", station.StationName(), err)
		}
	}
	return nil
}

// createStationFromConfig creates the appropriate weather station based on device type
func createStationFromConfig(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) (weatherstations.WeatherStation, error) {
	// Load configuration to determine device type
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	// Find the device configuration
	var deviceConfig *config.DeviceData
	for _, device := range cfgData.Devices {
		if device.Name == deviceName {
			deviceConfig = &device
			break
		}
	}

	if deviceConfig == nil {
		return nil, fmt.Errorf("device [%s] not found in configuration", deviceName)
	}

	switch deviceConfig.Type {
	case "campbellscientific":
		log.Infof("Initializing Campbell Scientific weather station [%v]", deviceName)
		return campbell.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	case "davis":
		log.Infof("Initializing Davis weather station [%v]", deviceName)
		return davis.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	case "snowgauge":
		log.Infof("Initializing snow gauge [%v]", deviceName)
		return snowgauge.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	default:
		return nil, fmt.Errorf("unknown weather station type: %s", deviceConfig.Type)
	}
}
