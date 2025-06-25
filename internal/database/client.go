package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"go.uber.org/zap"
)

// Client holds the connection to a TimescaleDB database
type Client struct {
	config *types.Config
	DB     *gorm.DB // Exported so it can be accessed from other packages
	logger *zap.SugaredLogger
}

// NewClient creates a new database client
func NewClient(c *types.Config, logger *zap.SugaredLogger) *Client {
	return &Client{
		config: c,
		logger: logger,
	}
}

// Connect connects to the TimescaleDB database
func (c *Client) Connect() error {
	var err error

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
	c.DB, err = gorm.Open(postgres.Open(c.config.Storage.TimescaleDB.ConnectionString), config)
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}
	log.Info("TimescaleDB connection successful")

	return nil
}

// ConnectToTimescaleDB is an alias for Connect for backward compatibility
func (c *Client) ConnectToTimescaleDB() error {
	return c.Connect()
}

// ValidatePullFromStation validates that the station name exists in config
func (c *Client) ValidatePullFromStation(pullFromDevice string) bool {
	if len(c.config.Devices) > 0 {
		for _, station := range c.config.Devices {
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

	return db, nil
}
