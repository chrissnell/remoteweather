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
	ConnectionString string `yaml:"connection-string"`
}

// TimescaleDBStorage holds the configuration for a TimescaleDB storage backend
type TimescaleDBStorage struct {
	TimescaleDBConn *gorm.DB
}

// We declare the Tabler interface for purposes of customizing the table name in the DB
type Tabler interface {
	TableName() string
}

// BucketReading holds a reading for a given timestamp
type BucketReading struct {
	Bucket time.Time `gorm:"column:bucket"`
	Reading
}

// We implement the Tabler interface for the Reading struct
func (Reading) TableName() string {
	return "weather"
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to TimescaleDB
func (t *TimescaleDBStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting TimescaleDB storage engine...")
	readingChan := make(chan Reading, 10)
	go t.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (t *TimescaleDBStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
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
func (t *TimescaleDBStorage) StoreReading(ctx context.Context, r Reading) {
	err := t.TimescaleDBConn.WithContext(ctx).Create(&r).Error
	if err != nil {
		log.Error("could not store reading:", err)
	}
}

// NewTimescaleDBStorage sets up a new Graphite storage backend
func NewTimescaleDBStorage(ctx context.Context, c *Config) (*TimescaleDBStorage, error) {

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
		return &TimescaleDBStorage{}, err
	}

	// Create the database table
	log.Info("creating database table...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createTableSQL).Error
	if err != nil {
		log.Warn("warning: could not create table in database")
		return &TimescaleDBStorage{}, err
	}

	// Create the TimescaleDB extension
	log.Info("creating TimescaleDB extension...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createExtensionSQL).Error
	if err != nil {
		log.Warn("warning: could not create TimescaleDB extension")
		return &TimescaleDBStorage{}, err
	}

	// Create the hypertable
	log.Info("creating hypertable...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createHypertableSQL).Error
	if err != nil {
		log.Warn("warning: could not create hypertable")
		return &TimescaleDBStorage{}, err
	}

	// Create the custom data type used to compute circular average
	log.Info("creating circular average custom data type")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgStateTypeSQL).Error
	if err != nil {
		// Postgres does not support "IF EXISTS" clause when creating new types,
		// so this will generate an error if the type already exists.  Unfortunately,
		// we can't simply delete and re-create the type because there are functions
		// that depend on it and we'd have to delete them, too.  So, we'll just
		// log an error and continue.
		log.Warn("Unable to create circular average custom data type, probably because it already exists.",
			"This warning just for informational purposes and you can safely ignore this message.",
			err)
	}

	// Create the circular average state accumulating function
	log.Info("creating circular average state accumulating function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgStateFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state accumulating function")
		return &TimescaleDBStorage{}, err
	}

	// Create the circular average state combiner function
	log.Info("creating circular average state combiner function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgCombinerFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state combiner function")
		return &TimescaleDBStorage{}, err
	}

	// Create the circular average finalizer function
	log.Info("creating circular average state finalizer function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgFinalizerFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state finalizer function")
		return &TimescaleDBStorage{}, err
	}

	// Create the circular average aggregate function
	log.Info("creating circular average state aggregate function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgAggregateFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state aggregate function")
		return &TimescaleDBStorage{}, err
	}

	// Create the 1m view
	log.Info("creating 1m view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1mViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1m view")
		return &TimescaleDBStorage{}, err
	}

	// Create the 5m view
	log.Info("creating 5m view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create5mViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 5m view")
		return &TimescaleDBStorage{}, err
	}

	// Create the 1h view
	log.Info("creating 1h view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1hViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1h view")
		return &TimescaleDBStorage{}, err
	}

	// Create the 1d view
	log.Info("Creating 1d view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1dViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1d view")
		return &TimescaleDBStorage{}, err
	}

	// There is no updating of views in PostgreSQL, so we have to drop the rain-since-midnight
	// view if it exists
	log.Info("Dropping the rain-since-midnight view if it exists...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(dropRainSinceMidnightViewSQL).Error
	if err != nil {
		log.Warn("warning: could not drop rain-since-midnight view")
		return &TimescaleDBStorage{}, err
	}

	// Add the rain-since-midnight view
	log.Info("Adding rain-since-midnight view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createRainSinceMidnightViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create rain-since-midnight view")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1m aggregation policy
	log.Info("Adding 1m aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1mSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1m aggregation policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 5m aggregation policy
	log.Info("Adding 5m aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy5mSQL).Error
	if err != nil {
		log.Warn("warning: could not add 5m aggregation policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1h aggregation policy
	log.Info("Adding 1h aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1hSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1h aggregation policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1d aggregation policy
	log.Info("Adding 1d aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1dSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1d aggregation policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the hypertable retention policy
	log.Info("Adding hypertable retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy).Error
	if err != nil {
		log.Warn("warning: could not add hypertable retention policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1m continuous aggregate retention policy
	log.Info("Adding 1m continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1m).Error
	if err != nil {
		log.Warn("warning: could not add 1m continous aggregate retention policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 5m continuous aggregate retention policy
	log.Info("Adding 5m continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy5m).Error
	if err != nil {
		log.Warn("warning: could not add 5m continous aggregate retention policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1h continuous aggregate retention policy
	log.Info("Adding 1h continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1h).Error
	if err != nil {
		log.Warn("warning: could not add 1h continous aggregate retention policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the 1d continuous aggregate retention policy
	log.Info("Adding 1d continuous aggregate retention policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1d).Error
	if err != nil {
		log.Warn("warning: could not add 1d continous aggregate retention policy")
		return &TimescaleDBStorage{}, err
	}

	// Add the snowfall delta function
	log.Info("Adding snowfall delta function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowDeltaFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not add snowfall delta function")
		return &TimescaleDBStorage{}, err
	}

	// Add the snowfall season total function
	log.Info("Adding snowfall season total function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowSeasonTotalSQL).Error
	if err != nil {
		log.Warn("warning: could not add snowfall season total function")
		return &TimescaleDBStorage{}, err
	}

	// Add the snowfall season total function
	log.Info("Adding snowfall storm total function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowStormTotalSQL).Error
	if err != nil {
		log.Warn("warning: could not add snowfall storm total function")
		return &TimescaleDBStorage{}, err
	}

	return &t, nil
}
