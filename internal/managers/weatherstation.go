package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/interfaces"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/internal/weatherstations/airgradient"
	"github.com/chrissnell/remoteweather/internal/weatherstations/ambientcustomized"
	"github.com/chrissnell/remoteweather/internal/weatherstations/campbell"
	"github.com/chrissnell/remoteweather/internal/weatherstations/davis"
	"github.com/chrissnell/remoteweather/internal/weatherstations/grpcreceiver"
	"github.com/chrissnell/remoteweather/internal/weatherstations/snowgauge"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// NewWeatherStationManager creates a WeatherStationManager object, populated with all configured weather stations
func NewWeatherStationManager(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, distributor chan types.Reading, logger *zap.SugaredLogger) (interfaces.WeatherStationManager, error) {
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
		stations:       make(map[string]weatherstations.WeatherStation),
	}

	// Create weather stations directly from config data (only for enabled devices)
	for _, deviceConfig := range cfgData.Devices {
		if !deviceConfig.Enabled {
			logger.Infof("Skipping disabled device [%s]", deviceConfig.Name)
			continue
		}
		station, err := createStationFromConfig(ctx, wg, configProvider, deviceConfig.Name, distributor, logger)
		if err != nil {
			return nil, fmt.Errorf("error creating weather station [%s]: %w", deviceConfig.Name, err)
		}
		wsm.stations[deviceConfig.Name] = station
	}

	return wsm, nil
}

type weatherStationManager struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	distributor    chan types.Reading
	logger         *zap.SugaredLogger
	stations       map[string]weatherstations.WeatherStation
	mu             sync.RWMutex
}

func (w *weatherStationManager) StartWeatherStations() error {
	w.logger.Info("Weather station manager started (clean orchestration)")
	for name, station := range w.stations {
		w.logger.Infof("Starting weather station [%v]...", name)
		if err := station.StartWeatherStation(); err != nil {
			return fmt.Errorf("failed to start weather station [%s]: %w", name, err)
		}
	}
	return nil
}

// AddWeatherStation adds a new weather station dynamically
func (w *weatherStationManager) AddWeatherStation(deviceName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if station already exists
	if _, exists := w.stations[deviceName]; exists {
		return fmt.Errorf("weather station %s already exists", deviceName)
	}

	// Check if device is enabled
	device, err := w.configProvider.GetDevice(deviceName)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", deviceName, err)
	}
	if !device.Enabled {
		return fmt.Errorf("cannot add disabled device %s", deviceName)
	}

	station, err := createStationFromConfig(w.ctx, w.wg, w.configProvider, deviceName, w.distributor, w.logger)
	if err != nil {
		return fmt.Errorf("error creating weather station [%s]: %w", deviceName, err)
	}

	w.stations[deviceName] = station

	// Start the station
	if err := station.StartWeatherStation(); err != nil {
		delete(w.stations, deviceName)
		return fmt.Errorf("failed to start weather station [%s]: %w", deviceName, err)
	}

	w.logger.Infof("Added and started weather station: %s", deviceName)
	return nil
}

// RemoveWeatherStation removes a weather station dynamically
func (w *weatherStationManager) RemoveWeatherStation(deviceName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	station, exists := w.stations[deviceName]
	if !exists {
		return fmt.Errorf("weather station %s not found", deviceName)
	}

	// Stop the weather station
	if err := station.StopWeatherStation(); err != nil {
		w.logger.Errorf("Error stopping weather station %s: %v", deviceName, err)
		// Continue with removal even if stop failed
	}

	// Remove from stations map
	delete(w.stations, deviceName)

	w.logger.Infof("Removed and stopped weather station: %s", deviceName)
	return nil
}

// ReloadWeatherStationsConfig reloads weather station configuration dynamically
func (w *weatherStationManager) ReloadWeatherStationsConfig() error {
	// Load new configuration
	cfgData, err := w.configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("could not load configuration: %v", err)
	}

	// Track what stations should be active (only enabled devices)
	shouldBeActive := make(map[string]bool)
	for _, deviceConfig := range cfgData.Devices {
		if deviceConfig.Enabled {
			shouldBeActive[deviceConfig.Name] = true
		}
	}

	// Remove stations that should no longer be active
	for name := range w.stations {
		if !shouldBeActive[name] {
			if err := w.RemoveWeatherStation(name); err != nil {
				w.logger.Errorf("Failed to remove weather station %s: %v", name, err)
			}
		}
	}

	// Add stations that should be active but aren't
	for name := range shouldBeActive {
		if _, exists := w.stations[name]; !exists {
			if err := w.AddWeatherStation(name); err != nil {
				w.logger.Errorf("Failed to add weather station %s: %v", name, err)
			}
		}
	}

	return nil
}

// GetStation retrieves a weather station by name.
// Returns nil if the station does not exist.
// This method is safe for concurrent use.
func (w *weatherStationManager) GetStation(deviceName string) weatherstations.WeatherStation {
	w.mu.RLock()
	defer w.mu.RUnlock()

	station, exists := w.stations[deviceName]
	if !exists {
		return nil
	}
	return station
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

	// Check if device is enabled
	if !deviceConfig.Enabled {
		return nil, fmt.Errorf("device [%s] is disabled", deviceName)
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
	case "ambient-customized":
		log.Infof("Initializing ambient-customized weather station [%v]", deviceName)
		return ambientcustomized.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	case "airgradient":
		log.Infof("Initializing AirGradient weather station [%v]", deviceName)
		return airgradient.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	case "grpcreceiver":
		log.Infof("Initializing gRPC receiver weather station [%v]", deviceName)
		return grpcreceiver.NewStation(ctx, wg, configProvider, deviceName, distributor, logger), nil
	default:
		return nil, fmt.Errorf("unknown weather station type: %s", deviceConfig.Type)
	}
}
