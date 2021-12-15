package main

import (
	"context"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TimescaleDBConfig describes the YAML-provided configuration for a TimescaleDB
// storage backend
type TimescaleDBConfig struct {
	ConnectionString string `yaml:"connection_string"`
}

// TimescaleDBStorage holds the configuration for a TimescaleDB storage backend
type TimescaleDBStorage struct {
	TimescaleDBConn *gorm.DB
}

// We declare the Tabler interface for purposes of customizing the table name in the DB
type Tabler interface {
	TableName() string
}

// We implement the Tabler interface for the Reading struct
func (Reading) TableName() string {
	return "weather"
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to TimescaleDB
func (t TimescaleDBStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting TimescaleDB storage engine...")
	readingChan := make(chan Reading, 10)
	go t.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (t TimescaleDBStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			t.StoreReading(ctx, r)
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in TimescaleDB
func (t TimescaleDBStorage) StoreReading(ctx context.Context, r Reading) {
	err := t.TimescaleDBConn.WithContext(ctx).Create(&r).Error
	if err != nil {
		log.Error("could not store reading:", err)
	}
}

// NewTimescaleDBStorage sets up a new Graphite storage backend
func NewTimescaleDBStorage(ctx context.Context, c *Config) (TimescaleDBStorage, error) {

	var err error
	t := TimescaleDBStorage{}

	log.Info("connecting to TimescaleDB...")
	t.TimescaleDBConn, err = gorm.Open(postgres.Open(c.Storage.TimescaleDB.ConnectionString), &gorm.Config{})
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return TimescaleDBStorage{}, err
	}

	// Create the database table
	log.Info("creating database table...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createTableSQL).Error
	if err != nil {
		log.Warn("warning: could not create table in database")
		return TimescaleDBStorage{}, err
	}

	// Create the TimescaleDB extension
	log.Info("creating TimescaleDB extension...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createExtensionSQL).Error
	if err != nil {
		log.Warn("warning: could not create TimescaleDB extension")
		return TimescaleDBStorage{}, err
	}

	// Create the hypertable
	log.Info("creating hypertable...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createHypertableSQL).Error
	if err != nil {
		log.Warn("warning: could not create hypertable")
		return TimescaleDBStorage{}, err
	}

	// Create the 5m view
	log.Info("creating 5m view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create5mViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 5m view")
		return TimescaleDBStorage{}, err
	}

	// Create the 1h view
	log.Info("creating 1h view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1hViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1h view")
		return TimescaleDBStorage{}, err
	}

	// Create the 1d view
	log.Info("Creating 1d view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1dViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1d view")
		return TimescaleDBStorage{}, err
	}

	return t, nil
}
