package main

import (
	"context"
	"fmt"
	"sync"
)

// StorageManager holds our active storage backends
type StorageManager struct {
	Engines            []StorageEngine
	ReadingDistributor chan Reading
}

// StorageEngine holds a backend storage engine's interface as well as
// a channel for passing readings to the engine
type StorageEngine struct {
	Engine StorageEngineInterface
	C      chan<- Reading
}

// StorageEngineInterface is an interface that provides a few standardized
// methods for various storage backends
type StorageEngineInterface interface {
	StartStorageEngine(context.Context, *sync.WaitGroup) chan<- Reading
}

// NewStorageManager creats a StorageManager object, populated with all configured
// StorageEngines
func NewStorageManager(ctx context.Context, wg *sync.WaitGroup, c *Config) (*StorageManager, error) {
	var err error

	s := StorageManager{}

	// Initialize our channel for passing metrics to the reading distributor
	s.ReadingDistributor = make(chan Reading, 20)

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

	if c.Storage.InfluxDB.Host != "" {
		err = s.AddEngine(ctx, wg, "influxdb", c)
		if err != nil {
			return &s, fmt.Errorf("could not add InfluxDB storage backend: %v", err)
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

// AddEngine adds a new StorageEngine of name engineName to our Storage object
func (s *StorageManager) AddEngine(ctx context.Context, wg *sync.WaitGroup, engineName string, c *Config) error {
	var err error

	switch engineName {
	case "timescaledb":
		se := StorageEngine{}
		se.Engine, err = NewTimescaleDBStorage(ctx, c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
	case "influxdb":
		se := StorageEngine{}
		se.Engine, err = NewInfluxDBStorage(c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)

	case "grpc":
		se := StorageEngine{}
		se.Engine, err = NewGRPCStorage(ctx, c)
		if err != nil {
			return err
		}
		se.C = se.Engine.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)

	case "aprs":
		se := StorageEngine{}
		se.Engine, err = NewAPRSStorage(c)
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
func (s *StorageManager) startReadingDistributor(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-s.ReadingDistributor:
			for _, e := range s.Engines {
				e.C <- r
			}
		case <-ctx.Done():
			log.Info("cancellation request received.  Cancelling reading distributor.")
			return nil
		}
	}
}
