package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/storage"
	"github.com/chrissnell/remoteweather/internal/storage/aprs"
	"github.com/chrissnell/remoteweather/internal/storage/grpcstream"
	"github.com/chrissnell/remoteweather/internal/storage/timescaledb"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// StorageManager holds our active storage backends
type StorageManager struct {
	Engines            []StorageEngine
	ReadingDistributor chan types.Reading
}

// StorageEngine holds a backend storage engine's interface as well as
// a channel for passing readings to the engine
type StorageEngine struct {
	Name   string
	Engine storage.StorageEngineInterface
	C      chan<- types.Reading
}

// NewStorageManager creates a StorageManager object, populated with all configured StorageEngines
func NewStorageManager(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider) (*StorageManager, error) {
	var err error

	s := StorageManager{}

	// Initialize our channel for passing metrics to the reading distributor
	s.ReadingDistributor = make(chan types.Reading, 20)

	// Start our reading distributor to distribute received readings to storage
	// backends
	go s.startReadingDistributor(ctx, wg)

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return &s, fmt.Errorf("could not load configuration: %v", err)
	}

	// Check the configuration file for various supported storage backends
	// and enable them if found

	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.GetConnectionString() != "" {
		err = s.AddEngine(ctx, wg, "timescaledb", configProvider)
		if err != nil {
			return &s, fmt.Errorf("could not add TimescaleDB storage backend: %v", err)
		}
	}

	if cfgData.Storage.GRPC != nil && cfgData.Storage.GRPC.Port != 0 {
		err = s.AddEngine(ctx, wg, "grpc", configProvider)
		if err != nil {
			return &s, fmt.Errorf("could not add gRPC storage backend: %v", err)
		}
	}

	// Check for APRS configuration in storage configs + device configs
	if cfgData.Storage.APRS != nil && cfgData.Storage.APRS.Server != "" {
		// Also check if we have devices with APRS enabled
		devices, err := configProvider.GetDevices()
		if err == nil && len(devices) > 0 {
			// Check if at least one device has APRS enabled
			for _, device := range devices {
				if device.APRSEnabled && device.APRSCallsign != "" {
					err = s.AddEngine(ctx, wg, "aprs", configProvider)
					if err != nil {
						return &s, fmt.Errorf("could not add APRS storage backend: %v", err)
					}
					break
				}
			}
		}
	}

	return &s, nil
}

// GetReadingDistributor returns the reading distributor channel
func (s *StorageManager) GetReadingDistributor() chan types.Reading {
	return s.ReadingDistributor
}

// AddEngine adds a new StorageEngine of name engineName to our Storage object
func (s *StorageManager) AddEngine(ctx context.Context, wg *sync.WaitGroup, engineName string, configProvider config.ConfigProvider) error {
	var err error

	// Check if engine already exists
	for _, engine := range s.Engines {
		if engine.Name == engineName {
			return fmt.Errorf("storage engine %s already exists", engineName)
		}
	}

	switch engineName {
	case "timescaledb":
		se := StorageEngine{Name: engineName}
		se.Engine, err = timescaledb.New(ctx, configProvider)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
		log.Infof("Added TimescaleDB storage engine")
	case "grpc":
		se := StorageEngine{Name: engineName}
		se.Engine, err = grpcstream.New(ctx, configProvider)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
		log.Infof("Added gRPC storage engine")
	case "aprs":
		se := StorageEngine{Name: engineName}
		se.Engine, err = aprs.New(configProvider)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
		log.Infof("Added APRS storage engine")
	}

	return nil
}

// RemoveEngine removes a storage engine by name
func (s *StorageManager) RemoveEngine(engineName string) error {
	for i, engine := range s.Engines {
		if engine.Name == engineName {
			// Close the channel to signal shutdown
			close(engine.C)

			// Remove from slice
			s.Engines = append(s.Engines[:i], s.Engines[i+1:]...)
			log.Infof("Removed storage engine: %s", engineName)
			return nil
		}
	}
	return fmt.Errorf("storage engine %s not found", engineName)
}

// ReloadStorageConfig reloads storage configuration dynamically
func (s *StorageManager) ReloadStorageConfig(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider) error {
	// Load new configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("could not load configuration: %v", err)
	}

	// Track what engines should be active
	shouldBeActive := make(map[string]bool)

	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.GetConnectionString() != "" {
		shouldBeActive["timescaledb"] = true
	}
	if cfgData.Storage.GRPC != nil && cfgData.Storage.GRPC.Port != 0 {
		shouldBeActive["grpc"] = true
	}

	// Check for APRS configuration in storage configs + device configs
	storageConfig, err := configProvider.GetStorageConfig()
	if err == nil && storageConfig.APRS != nil && storageConfig.APRS.Server != "" {
		// Also check if we have devices with APRS enabled
		devices, err := configProvider.GetDevices()
		if err == nil && len(devices) > 0 {
			// Check if at least one device has APRS enabled
			for _, device := range devices {
				if device.APRSEnabled && device.APRSCallsign != "" {
					shouldBeActive["aprs"] = true
					break
				}
			}
		}
	}

	// Remove engines that should no longer be active
	for i := len(s.Engines) - 1; i >= 0; i-- {
		engine := s.Engines[i]
		if !shouldBeActive[engine.Name] {
			if err := s.RemoveEngine(engine.Name); err != nil {
				log.Errorf("Failed to remove storage engine %s: %v", engine.Name, err)
			}
		}
	}

	// Add engines that should be active but aren't
	currentEngines := make(map[string]bool)
	for _, engine := range s.Engines {
		currentEngines[engine.Name] = true
	}

	for engineName := range shouldBeActive {
		if !currentEngines[engineName] {
			if err := s.AddEngine(ctx, wg, engineName, configProvider); err != nil {
				log.Errorf("Failed to add storage engine %s: %v", engineName, err)
			}
		}
	}

	return nil
}

// startReadingDistributor receives readings from gatherers and fans them out to the various
// storage backends
func (s *StorageManager) startReadingDistributor(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	readingCount := 0
	for {
		select {
		case r := <-s.ReadingDistributor:
			readingCount++
			log.Debugf("Reading distributor received reading #%d from [%s] (%s): temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph",
				readingCount, r.StationName, r.StationType, r.OutTemp, r.OutHumidity, r.WindSpeed)

			if len(s.Engines) == 0 {
				log.Debug("No storage engines configured - reading discarded")
			} else {
				log.Debugf("Distributing reading to %d storage engine(s)", len(s.Engines))
				for _, e := range s.Engines {
					e.C <- r
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
