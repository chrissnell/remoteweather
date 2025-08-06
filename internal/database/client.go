// Package database provides database client functionality for TimescaleDB connections.
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// Client holds the connection to a TimescaleDB database
type Client struct {
	configProvider config.ConfigProvider
	DB             *gorm.DB // Exported so it can be accessed from other packages
	logger         *zap.SugaredLogger
}

// NewClient creates a new database client
func NewClient(configProvider config.ConfigProvider, logger *zap.SugaredLogger) *Client {
	return &Client{
		configProvider: configProvider,
		logger:         logger,
	}
}

// Connect connects to the TimescaleDB database
func (c *Client) Connect() error {
	var err error

	// Load configuration
	cfgData, err := c.configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading configuration: %v", err)
	}

	if cfgData.Storage.TimescaleDB == nil {
		return fmt.Errorf("TimescaleDB configuration not found")
	}

	connectionString := cfgData.Storage.TimescaleDB.GetConnectionString()
	if connectionString == "" {
		return fmt.Errorf("TimescaleDB connection string not configured")
	}

	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(log.GetZapLogger()),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: false,       // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Use colors
		},
	)

	config := &gorm.Config{
		Logger: dbLogger,
	}

	log.Info("connecting to TimescaleDB...")
	c.DB, err = gorm.Open(postgres.Open(connectionString), config)
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}
	log.Info("TimescaleDB connection successful")

	// Configure connection pool for better performance with parallel queries
	sqlDB, err := c.DB.DB()
	if err != nil {
		return err
	}
	
	// Set maximum number of open connections
	// This should be high enough to handle parallel queries
	sqlDB.SetMaxOpenConns(25)
	
	// Set maximum number of idle connections
	sqlDB.SetMaxIdleConns(10)
	
	// Set maximum lifetime of a connection
	// This helps with load balancing and prevents stale connections
	sqlDB.SetConnMaxLifetime(time.Hour)

	return nil
}

// ConnectToTimescaleDB is an alias for Connect for backward compatibility
func (c *Client) ConnectToTimescaleDB() error {
	return c.Connect()
}

// ValidatePullFromStation validates that the station name exists in config
func (c *Client) ValidatePullFromStation(pullFromDevice string) bool {
	cfgData, err := c.configProvider.LoadConfig()
	if err != nil {
		c.logger.Errorf("error loading configuration for validation: %v", err)
		return false
	}

	if len(cfgData.Devices) > 0 {
		for _, station := range cfgData.Devices {
			if station.Name == pullFromDevice {
				return true
			}
		}
	}
	return false
}

// GetReadingsFromTimescaleDB retrieves readings from TimescaleDB
func (c *Client) GetReadingsFromTimescaleDB(pullFromDevice string) (FetchedBucketReading, error) {
	var br FetchedBucketReading

	if err := c.DB.Table("weather_1m").Where("stationname=? AND bucket > NOW() - INTERVAL '2 minutes'", pullFromDevice).Limit(1).Find(&br).Error; err != nil {
		return FetchedBucketReading{}, fmt.Errorf("error querying database for latest readings: %+v", err)
	}

	return br, nil
}

// CreateConnection is a helper function to create a database connection with standard GORM configuration
func CreateConnection(connectionString string) (*gorm.DB, error) {
	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(log.GetZapLogger()),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	log.Info("connecting to TimescaleDB...")
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{Logger: dbLogger})
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return nil, err
	}

	// Configure connection pool for better performance with parallel queries
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	
	// Set maximum number of open connections
	// This should be high enough to handle parallel queries
	sqlDB.SetMaxOpenConns(25)
	
	// Set maximum number of idle connections
	sqlDB.SetMaxIdleConns(10)
	
	// Set maximum lifetime of a connection
	// This helps with load balancing and prevents stale connections
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
