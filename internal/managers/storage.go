package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/storage"
	"github.com/chrissnell/remoteweather/internal/storage/aprs"
	"github.com/chrissnell/remoteweather/internal/storage/grpc"
	"github.com/chrissnell/remoteweather/internal/storage/timescaledb"
	"github.com/chrissnell/remoteweather/internal/types"
)

// StorageManager holds our active storage backends
type StorageManager struct {
	Engines            []StorageEngine
	ReadingDistributor chan types.Reading
}

// StorageEngine holds a backend storage engine's interface as well as
// a channel for passing readings to the engine
type StorageEngine struct {
	Engine storage.StorageEngineInterface
	C      chan<- types.Reading
}

// NewStorageManager creates a StorageManager object, populated with all configured StorageEngines
func NewStorageManager(ctx context.Context, wg *sync.WaitGroup, c *types.Config) (*StorageManager, error) {
	var err error

	s := StorageManager{}

	// Initialize our channel for passing metrics to the reading distributor
	s.ReadingDistributor = make(chan types.Reading, 20)

	// Start our reading distributor to distribute received readings to storage
	// backends
	go s.startReadingDistributor(ctx, wg)

	// Check the configuration file for various supported storage backends
	// and enable them if found

	if c.Storage.TimescaleDB.ConnectionString != "" {
		err = s.AddEngine(ctx, wg, "timescaledb", c)
		if err != nil {
			return &s, fmt.Errorf("could not add TimescaleDB storage backend: %v", err)
		}
	}

	if c.Storage.GRPC.Port != 0 {
		err = s.AddEngine(ctx, wg, "grpc", c)
		if err != nil {
			return &s, fmt.Errorf("could not add gRPC storage backend: %v", err)
		}
	}

	if c.Storage.APRS.Callsign != "" {
		err = s.AddEngine(ctx, wg, "aprs", c)
		if err != nil {
			return &s, fmt.Errorf("could not add APRS storage backend: %v", err)
		}
	}

	return &s, nil
}

// GetReadingDistributor returns the reading distributor channel
func (s *StorageManager) GetReadingDistributor() chan types.Reading {
	return s.ReadingDistributor
}

// AddEngine adds a new StorageEngine of name engineName to our Storage object
func (s *StorageManager) AddEngine(ctx context.Context, wg *sync.WaitGroup, engineName string, c *types.Config) error {
	var err error

	switch engineName {
	case "timescaledb":
		se := StorageEngine{}
		se.Engine, err = timescaledb.New(ctx, c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
	case "grpc":
		se := StorageEngine{}
		se.Engine, err = grpc.New(ctx, c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
	case "aprs":
		se := StorageEngine{}
		se.Engine, err = aprs.New(c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
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
