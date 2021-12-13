package main

import (
	"context"
	"log"
	"sync"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
)

// TimescaleDBConfig describes the YAML-provided configuration for a TimescaleDB
// storage backend
type TimescaleDBConfig struct {
	ConnectionString string `yaml:"connection_string"`
}

// TimescaleDBStorage holds the configuration for a TimescaleDB storage backend
type TimescaleDBStorage struct {
	TimescaleDBConn *sqlx.DB
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to TimescaleDB
func (t TimescaleDBStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
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
			err := t.StoreReading(ctx, r)
			if err != nil {
				log.Println(err)
			}
		case <-ctx.Done():
			log.Println("Cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in TimescaleDB
func (t TimescaleDBStorage) StoreReading(ctx context.Context, r Reading) error {

	_, err := t.TimescaleDBConn.NamedExecContext(ctx, insertDataSQL, &r)
	if err != nil {
		return err
	}

	return nil
}

// NewTimescaleDBStorage sets up a new Graphite storage backend
func NewTimescaleDBStorage(ctx context.Context, c *Config) (TimescaleDBStorage, error) {

	var err error
	t := TimescaleDBStorage{}

	log.Println("Connecting to TimescaleDB...")
	t.TimescaleDBConn, err = sqlx.Open("pgx", c.Storage.TimescaleDB.ConnectionString)
	if err != nil {
		log.Println("Warning: unable to create a TimescaleDB connection:", err)
		return TimescaleDBStorage{}, err
	}

	// Create the database table
	log.Println("Creating database table...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, createTableSQL)
	if err != nil {
		log.Println("Warning: could not create table in database")
		return TimescaleDBStorage{}, err
	}

	// Create the TimescaleDB extension
	log.Println("Creating TimescaleDB extension...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, createExtensionSQL)
	if err != nil {
		log.Println("Warning: could not create the TimescaleDB extension in the database")
		return TimescaleDBStorage{}, err
	}

	// Create the hypertable
	log.Println("Creating hypertable...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, createHypertableSQL)
	if err != nil {
		log.Println("Warning: could not create hypertable in database")
		return TimescaleDBStorage{}, err
	}

	// Create the 5m view
	log.Println("Creating 5m view...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, create5mViewSQL)
	if err != nil {
		log.Println("Warning: could not create 5-minute view in database")
		return TimescaleDBStorage{}, err
	}

	// Create the 1h view
	log.Println("Creating 1h view...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, create1hViewSQL)
	if err != nil {
		log.Println("Warning: could not create 1-hour view in database")
		return TimescaleDBStorage{}, err
	}

	// Create the 1d view
	log.Println("Creating 1d view...")
	_, err = t.TimescaleDBConn.ExecContext(ctx, create1dViewSQL)
	if err != nil {
		log.Println("Warning: could not create 1-day view in database")
		return TimescaleDBStorage{}, err
	}

	return t, nil
}
