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
	"go.uber.org/zap"
)

// WeatherStationManager interface for the weather station manager
type WeatherStationManager interface {
	StartWeatherStations() error
}

// NewWeatherStationManager creates a WeatherStationManager object, populated with all configured weather stations
func NewWeatherStationManager(ctx context.Context, wg *sync.WaitGroup, c *types.Config, distributor chan types.Reading, logger *zap.SugaredLogger) (WeatherStationManager, error) {
	// Create the actual weather station manager using clean orchestration
	actualManager, err := newWeatherStationManagerFromConfig(ctx, wg, c, distributor, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create weather station manager: %w", err)
	}

	return &weatherStationManager{
		ctx:           ctx,
		wg:            wg,
		config:        c,
		distributor:   distributor,
		logger:        logger,
		actualManager: actualManager,
	}, nil
}

type weatherStationManager struct {
	ctx           context.Context
	wg            *sync.WaitGroup
	config        *types.Config
	distributor   chan types.Reading
	logger        *zap.SugaredLogger
	actualManager *stationManagerImpl
}

func (w *weatherStationManager) StartWeatherStations() error {
	w.logger.Info("Weather station manager started (clean orchestration)")
	return w.actualManager.StartWeatherStations()
}

// stationManagerImpl holds our active weather station backends
type stationManagerImpl struct {
	stations []weatherstations.WeatherStation
	logger   *zap.SugaredLogger
}

// newWeatherStationManagerFromConfig creates a weather station manager using clean orchestration
func newWeatherStationManagerFromConfig(ctx context.Context, wg *sync.WaitGroup, c *types.Config, distributor chan types.Reading, logger *zap.SugaredLogger) (*stationManagerImpl, error) {
	wsm := &stationManagerImpl{
		logger: logger,
	}

	for _, deviceConfig := range c.Devices {
		station, err := createStationFromConfig(ctx, wg, deviceConfig, distributor, logger)
		if err != nil {
			return nil, fmt.Errorf("error creating weather station [%s]: %w", deviceConfig.Name, err)
		}
		wsm.stations = append(wsm.stations, station)
	}

	return wsm, nil
}

// createStationFromConfig creates the appropriate weather station based on device type
func createStationFromConfig(ctx context.Context, wg *sync.WaitGroup, config types.DeviceConfig, distributor chan types.Reading, logger *zap.SugaredLogger) (weatherstations.WeatherStation, error) {
	switch config.Type {
	case "campbellscientific":
		log.Infof("Initializing Campbell Scientific weather station [%v]", config.Name)
		return campbell.NewStation(ctx, wg, config, distributor, logger), nil
	case "davis":
		log.Infof("Initializing Davis weather station [%v]", config.Name)
		return davis.NewStation(ctx, wg, config, distributor, logger), nil
	case "snowgauge":
		log.Infof("Initializing snow gauge [%v]", config.Name)
		return snowgauge.NewStation(ctx, wg, config, distributor, logger), nil
	default:
		return nil, fmt.Errorf("unknown weather station type: %s", config.Type)
	}
}

func (wsm *stationManagerImpl) StartWeatherStations() error {
	for _, station := range wsm.stations {
		wsm.logger.Infof("Starting weather station [%v]...", station.StationName())
		if err := station.StartWeatherStation(); err != nil {
			return fmt.Errorf("failed to start weather station [%s]: %w", station.StationName(), err)
		}
	}
	return nil
}
