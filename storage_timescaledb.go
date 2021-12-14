package main

import (
	"context"
	"log"
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
	log.Println("Starting TimescaleDB storage engine...")
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
			log.Println("Cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in TimescaleDB
func (t TimescaleDBStorage) StoreReading(ctx context.Context, r Reading) {
	t.TimescaleDBConn.WithContext(ctx).Create(&r)
}

// NewTimescaleDBStorage sets up a new Graphite storage backend
func NewTimescaleDBStorage(ctx context.Context, c *Config) (TimescaleDBStorage, error) {

	var err error
	t := TimescaleDBStorage{}

	log.Println("Connecting to TimescaleDB...")
	t.TimescaleDBConn, err = gorm.Open(postgres.Open(c.Storage.TimescaleDB.ConnectionString), &gorm.Config{})
	if err != nil {
		log.Println("Warning: unable to create a TimescaleDB connection:", err)
		return TimescaleDBStorage{}, err
	}

	// Create the database table
	log.Println("Creating database table...")
	t.TimescaleDBConn.WithContext(ctx).Exec(createTableSQL)
	if err != nil {
		log.Println("Warning: could not create table in database")
		return TimescaleDBStorage{}, err
	}

	// Create the TimescaleDB extension
	log.Println("Creating TimescaleDB extension...")
	t.TimescaleDBConn.WithContext(ctx).Exec(createExtensionSQL)

	// Create the hypertable
	log.Println("Creating hypertable...")
	t.TimescaleDBConn.WithContext(ctx).Exec(createHypertableSQL)

	// Create the 5m view
	log.Println("Creating 5m view...")
	t.TimescaleDBConn.WithContext(ctx).Exec(create5mViewSQL)

	// Create the 1h view
	log.Println("Creating 1h view...")
	t.TimescaleDBConn.WithContext(ctx).Exec(create1hViewSQL)

	// Create the 1d view
	log.Println("Creating 1d view...")
	t.TimescaleDBConn.WithContext(ctx).Exec(create1dViewSQL)

	return t, nil
}
