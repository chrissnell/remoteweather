package main

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(zapLogger),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	log.Info("connecting to TimescaleDB...")
	t.TimescaleDBConn, err = gorm.Open(postgres.Open(c.Storage.TimescaleDB.ConnectionString), &gorm.Config{Logger: dbLogger})
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

	// Add the 5m aggregation policy
	log.Info("Adding 5m aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy5mSQL).Error
	if err != nil {
		log.Warn("warning: could not add 5m aggregation policy")
		return TimescaleDBStorage{}, err
	}

	// Add the 1h aggregation policy
	log.Info("Adding 1h aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1hSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1h aggregation policy")
		return TimescaleDBStorage{}, err
	}

	// Add the 1d aggregation policy
	log.Info("Adding 1d aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1dSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1d aggregation policy")
		return TimescaleDBStorage{}, err
	}

	// Add the hypertable retention policy
	log.Info("Adding hypertable retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy).Error
	if err != nil {
		log.Warn("warning: could not add hypertable retention policy")
		return TimescaleDBStorage{}, err
	}

	// Add the 5m continuous aggregate retention policy
	log.Info("Adding 5m continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy5m).Error
	if err != nil {
		log.Warn("warning: could not add 5m continous aggregate retention policy")
		return TimescaleDBStorage{}, err
	}

	// Add the 1h continuous aggregate retention policy
	log.Info("Adding 1h continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1h).Error
	if err != nil {
		log.Warn("warning: could not add 1h continous aggregate retention policy")
		return TimescaleDBStorage{}, err
	}

	return t, nil
}
