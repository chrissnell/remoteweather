package timescaledb

import (
	"context"
	"sync"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"gorm.io/gorm"
)

// Storage holds the configuration for a TimescaleDB storage backend
type Storage struct {
	TimescaleDBConn *gorm.DB
}

// We declare the Tabler interface for purposes of customizing the table name in the DB
type Tabler interface {
	TableName() string
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to TimescaleDB
func (t *Storage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting TimescaleDB storage engine...")
	readingChan := make(chan types.Reading, 10)
	go t.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (t *Storage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan types.Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			err := t.StoreReading(r)
			if err != nil {
				log.Info("cancellation request recieved.  Cancelling readings processor.")
				return
			}
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in TimescaleDB
func (t *Storage) StoreReading(r types.Reading) error {
	err := t.TimescaleDBConn.Create(&r).Error
	if err != nil {
		log.Error("could not store reading:", err)
		return err
	}
	return nil
}

// New sets up a new TimescaleDB storage backend
func New(ctx context.Context, c *types.Config) (*Storage, error) {

	var err error
	t := Storage{}

	log.Info("connecting to TimescaleDB...")
	t.TimescaleDBConn, err = database.CreateConnection(c.Storage.TimescaleDB.ConnectionString)
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return &Storage{}, err
	}

	// Create the database table
	log.Info("creating database table...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createTableSQL).Error
	if err != nil {
		log.Warn("warning: could not create table in database")
		return &Storage{}, err
	}

	// Create the TimescaleDB extension
	log.Info("creating TimescaleDB extension...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createExtensionSQL).Error
	if err != nil {
		log.Warn("warning: could not create TimescaleDB extension")
		return &Storage{}, err
	}

	// Create the hypertable
	log.Info("creating hypertable...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createHypertableSQL).Error
	if err != nil {
		log.Warn("warning: could not create hypertable")
		return &Storage{}, err
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
		return &Storage{}, err
	}

	// Create the circular average state combiner function
	log.Info("creating circular average state combiner function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgCombinerFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state combiner function")
		return &Storage{}, err
	}

	// Create the circular average finalizer function
	log.Info("creating circular average state finalizer function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgFinalizerFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state finalizer function")
		return &Storage{}, err
	}

	// Create the circular average aggregate function
	log.Info("creating circular average state aggregate function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCircAvgAggregateFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not create circular average state aggregate function")
		return &Storage{}, err
	}

	// Create the 1m view
	log.Info("creating 1m view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1mViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1m view")
		return &Storage{}, err
	}

	// Create the 5m view
	log.Info("creating 5m view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create5mViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 5m view")
		return &Storage{}, err
	}

	// Create the 1h view
	log.Info("creating 1h view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1hViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1h view")
		return &Storage{}, err
	}

	// Create the 1d view
	log.Info("Creating 1d view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(create1dViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create 1d view")
		return &Storage{}, err
	}

	// There is no updating of views in PostgreSQL, so we have to drop the rain-since-midnight
	// view if it exists
	log.Info("Dropping the rain-since-midnight view if it exists...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(dropRainSinceMidnightViewSQL).Error
	if err != nil {
		log.Warn("warning: could not drop rain-since-midnight view")
		return &Storage{}, err
	}

	// Add the rain-since-midnight view
	log.Info("Adding rain-since-midnight view...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createRainSinceMidnightViewSQL).Error
	if err != nil {
		log.Warn("warning: could not create rain-since-midnight view")
		return &Storage{}, err
	}

	// Add the 1m aggregation policy
	log.Info("Adding 1m aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1mSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1m aggregation policy")
		return &Storage{}, err
	}

	// Add the 5m aggregation policy
	log.Info("Adding 5m aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy5mSQL).Error
	if err != nil {
		log.Warn("warning: could not add 5m aggregation policy")
		return &Storage{}, err
	}

	// Add the 1h aggregation policy
	log.Info("Adding 1h aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1hSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1h aggregation policy")
		return &Storage{}, err
	}

	// Add the 1d aggregation policy
	log.Info("Adding 1d aggregation policy...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addAggregationPolicy1dSQL).Error
	if err != nil {
		log.Warn("warning: could not add 1d aggregation policy")
		return &Storage{}, err
	}

	return &t, nil
}
