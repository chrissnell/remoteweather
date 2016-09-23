package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Storage holds our active storage backends
type Storage struct {
	Engines            []StorageEngine
	ReadingDistributor chan Reading
}

// StorageEngine holds a backend storage engine's interface as well as
// a channel for passing readings to the engine
type StorageEngine struct {
	I StorageEngineInterface
	C chan<- Reading
}

// StorageEngineInterface is an interface that provides a few standardized
// methods for various storage backends
type StorageEngineInterface interface {
	SendReading(Reading) error
	StartStorageEngine(context.Context, *sync.WaitGroup) chan<- Reading
}

// NewStorage creats a Storage object, populated with all configured
// StorageEngines
func NewStorage(ctx context.Context, wg *sync.WaitGroup, c *Config) (*Storage, error) {
	var err error

	s := Storage{}

	// Initialize our channel for passing metrics to the MetricDistributor
	s.ReadingDistributor = make(chan Reading, 20)

	// Start our reading distributor to distribute received readings to storage
	// backends
	go s.readingDistributor(ctx, wg)

	// Check the configuration file for various supported storage backends
	// and enable them if found

	if c.Storage.InfluxDB.Host != "" {
		err = s.AddEngine(ctx, wg, "influxdb", c)
		if err != nil {
			return &s, fmt.Errorf("Could not add InfluxDB storage backend: %v\n", err)
		}
	}

	// if c.Storage.Graphite.Host != "" {
	// 	err = s.AddEngine(ctx, wg, "graphite", c)
	// 	if err != nil {
	// 		return &s, fmt.Errorf("Could not add Graphite storage backend: %v\n", err)
	// 	}
	// }

	return &s, nil
}

// AddEngine adds a new StorageEngine of name engineName to our Storage object
func (s *Storage) AddEngine(ctx context.Context, wg *sync.WaitGroup, engineName string, c *Config) error {
	var err error

	switch engineName {
	case "influxdb":
		se := StorageEngine{}
		se.I, err = NewInfluxDBStorage(c)
		if err != nil {
			return err
		}
		se.C = se.I.StartStorageEngine(ctx, wg)
		s.Engines = append(s.Engines, se)
	}

	return nil
}

// readingDistributor receives readings from gatherers and fans them out to the various
// storage backends
func (s *Storage) readingDistributor(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-s.ReadingDistributor:
			for _, e := range s.Engines {
				log.Println("Reading Distributor :: Sending reading", r.Timestamp.Format(time.RFC822))
				e.C <- r
			}
		case <-ctx.Done():
			log.Println("Cancellation request received.  Cancelling reading distributor.")
			return nil
		}
	}
}
