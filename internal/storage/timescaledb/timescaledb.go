// Package timescaledb provides TimescaleDB storage backend for weather data persistence.
package timescaledb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/storage"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	"gorm.io/gorm"
)

// Storage holds the configuration for a TimescaleDB storage backend
type Storage struct {
	TimescaleDBConn *gorm.DB
}

// Note: TimescaleDBClient functionality has been moved to internal/database package.
// Use database.NewClient() for database operations like reading data and connection management.

// Tabler interface allows customizing the table name in the database
type Tabler interface {
	TableName() string
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to TimescaleDB
func (t *Storage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting TimescaleDB storage engine...")
	readingChan := make(chan types.Reading, 10)
	go storage.ProcessReadings(ctx, wg, readingChan, t.StoreReading, "TimescaleDB")
	return readingChan
}

// StoreReading stores a reading value in TimescaleDB
func (t *Storage) StoreReading(r types.Reading) error {
	err := t.TimescaleDBConn.Create(&r).Error
	if err != nil {
		log.Error("could not store reading:", err)
		return err
	}
	log.Debugf("TimescaleDB stored reading for station %s", r.StationName)
	return nil
}

func (t *Storage) CheckHealth(configProvider config.ConfigProvider) *config.StorageHealthData {
	if t.TimescaleDBConn == nil {
		return storage.CreateHealthData("unhealthy", "No database connection", fmt.Errorf("TimescaleDB connection is nil"))
	}

	sqlDB, err := t.TimescaleDBConn.DB()
	if err != nil {
		return storage.CreateHealthData("unhealthy", "Failed to get underlying database connection", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return storage.CreateHealthData("unhealthy", "Database ping failed", err)
	}

	var result int
	if err := t.TimescaleDBConn.Raw("SELECT 1").Scan(&result).Error; err != nil {
		return storage.CreateHealthData("unhealthy", "Database query test failed", err)
	}

	return storage.CreateHealthData("healthy", "TimescaleDB operational - ping: OK, query test: OK", nil)
}

// New sets up a new TimescaleDB storage backend
func New(ctx context.Context, configProvider config.ConfigProvider) (*Storage, error) {

	var err error
	t := Storage{}

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return &Storage{}, err
	}

	if cfgData.Storage.TimescaleDB == nil {
		return &Storage{}, fmt.Errorf("TimescaleDB configuration not found")
	}

	connectionString := cfgData.Storage.TimescaleDB.GetConnectionString()
	if connectionString == "" {
		return &Storage{}, fmt.Errorf("TimescaleDB connection string not configured")
	}

	log.Info("connecting to TimescaleDB...")
	t.TimescaleDBConn, err = database.CreateConnection(connectionString)
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

	// The today_rainfall view was removed in favor of the calculate_daily_rainfall() function
	// which is created via migration 004_add_daily_rainfall_function.up.sql
	// This provides better performance (45a3e98) and station-specific calculations

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

	// Add retention policies
	log.Info("Adding retention policy for raw data...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicySQL).Error
	if err != nil {
		log.Warn("warning: could not add retention policy for raw data")
		return &Storage{}, err
	}

	log.Info("Adding retention policy for 1m data...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1mSQL).Error
	if err != nil {
		log.Warn("warning: could not add retention policy for 1m data")
		return &Storage{}, err
	}

	log.Info("Adding retention policy for 5m data...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy5mSQL).Error
	if err != nil {
		log.Warn("warning: could not add retention policy for 5m data")
		return &Storage{}, err
	}

	log.Info("Adding retention policy for 1h data...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1hSQL).Error
	if err != nil {
		log.Warn("warning: could not add retention policy for 1h data")
		return &Storage{}, err
	}

	log.Info("Adding retention policy for 1d data...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRetentionPolicy1dSQL).Error
	if err != nil {
		log.Warn("warning: could not add retention policy for 1d data")
		return &Storage{}, err
	}

	// Add snow depth calculation functions
	log.Info("Adding snow depth calculations...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addSnowDepthCalculations).Error
	if err != nil {
		log.Warn("warning: could not add snow depth calculations")
		return &Storage{}, err
	}

	log.Info("Adding dual-threshold snow detection function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowDualThresholdSQL).Error
	if err != nil {
		log.Warn("warning: could not add dual-threshold snow detection function")
		return &Storage{}, err
	}

	log.Info("Adding 72h snow delta function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowDelta72hSQL).Error
	if err != nil {
		log.Warn("warning: could not add 72h snow delta function")
		return &Storage{}, err
	}

	log.Info("Adding 24h snow delta function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowDelta24hSQL).Error
	if err != nil {
		log.Warn("warning: could not add 24h snow delta function")
		return &Storage{}, err
	}

	log.Info("Adding midnight snow delta function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowDeltaSinceMidnightSQL).Error
	if err != nil {
		log.Warn("warning: could not add midnight snow delta function")
		return &Storage{}, err
	}

	log.Info("Adding simple positive delta function for seasonal calculations...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowSimplePositiveDeltaSQL).Error
	if err != nil {
		log.Warn("warning: could not add simple positive delta function")
		return &Storage{}, err
	}

	log.Info("Adding season snow total function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowSeasonTotalSQL).Error
	if err != nil {
		log.Warn("warning: could not add season snow total function")
		return &Storage{}, err
	}

	log.Info("Adding storm snow total function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowStormTotalSQL).Error
	if err != nil {
		log.Warn("warning: could not add storm snow total function")
		return &Storage{}, err
	}

	log.Info("Adding current snowfall rate function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCurrentSnowfallRateSQL).Error
	if err != nil {
		log.Warn("warning: could not add current snowfall rate function")
		return &Storage{}, err
	}

	// Snow cache table and refresh job
	log.Info("Creating snow totals cache table...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowCacheTableSQL).Error
	if err != nil {
		log.Warn("warning: could not create snow totals cache table")
		return &Storage{}, err
	}

	log.Info("Adding snow cache refresh function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createSnowCacheRefreshFunctionSQL).Error
	if err != nil {
		log.Warn("warning: could not add snow cache refresh function")
		return &Storage{}, err
	}

	log.Info("Adding snow cache refresh job...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addSnowCacheJobSQL).Error
	if err != nil {
		log.Warn("warning: could not add snow cache refresh job")
		return &Storage{}, err
	}

	log.Info("Adding storm rainfall total function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createRainStormTotalSQL).Error
	if err != nil {
		log.Warn("warning: could not add storm rainfall total function")
		return &Storage{}, err
	}

	log.Info("Adding current rainfall rate function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createCurrentRainfallRateSQL).Error
	if err != nil {
		log.Warn("warning: could not add current rainfall rate function")
		return &Storage{}, err
	}

	log.Info("Adding wind gust calculation function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createWindGustSQL).Error
	if err != nil {
		log.Warn("warning: could not add wind gust calculation function")
		return &Storage{}, err
	}

	// Add indexes to speed up our most common queries
	log.Info("Creating indexes...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createIndexesSQL).Error
	if err != nil {
		log.Warn("warning: could not create indexes")
		return &Storage{}, err
	}

	// Create rainfall summary table and functions for fast queries
	log.Info("Creating rainfall summary table...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createRainfallSummaryTableSQL).Error
	if err != nil {
		log.Warn("warning: could not create rainfall summary table")
	}

	log.Info("Creating update rainfall summary function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createUpdateRainfallSummarySQL).Error
	if err != nil {
		log.Warn("warning: could not create update rainfall summary function")
	}

	log.Info("Creating get rainfall with recent function...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(createGetRainfallWithRecentSQL).Error
	if err != nil {
		log.Warn("warning: could not create get rainfall with recent function")
	}

	// Add TimescaleDB job for updating rainfall summary
	log.Info("Adding rainfall summary update job...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec(addRainfallSummaryJobSQL).Error
	if err != nil {
		log.Warn("warning: could not add rainfall summary job")
	}

	// Initialize the rainfall summary with current data
	log.Info("Initializing rainfall summary...")
	err = t.TimescaleDBConn.WithContext(ctx).Exec("SELECT update_rainfall_summary()").Error
	if err != nil {
		log.Warn("warning: could not initialize rainfall summary")
	}

	// Start health monitoring
	storage.StartHealthMonitor(ctx, configProvider, "timescaledb", &t, 60*time.Second)

	log.Info("TimescaleDB connection successful")

	return &t, nil
}
