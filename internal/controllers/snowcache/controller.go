// Package snowcache provides a dedicated controller for snow cache refresh operations.
// This controller runs independently of the REST server and handles periodic snow
// accumulation calculations using the PELT-based statistical algorithm.
package snowcache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/snow"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// Controller manages the snow cache refresh lifecycle
type Controller struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	db             *sql.DB
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	calculator     *snow.Calculator
	stationName    string
	baseDistance   float64
	ticker         *time.Ticker
	stopChan       chan struct{}
}

// NewController creates a new snow cache controller
// Returns nil if snow is not enabled in the configuration
func NewController(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *sql.DB,
	configProvider config.ConfigProvider,
	logger *zap.SugaredLogger,
) (*Controller, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection required for snow cache controller")
	}

	// Load configuration to find snow-enabled website
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Load weather websites to check for snow configuration
	websites, err := configProvider.GetWeatherWebsites()
	if err != nil {
		return nil, fmt.Errorf("failed to load weather websites: %w", err)
	}

	// Find snow-enabled website and device
	var stationName string
	var baseDistance float64
	var found bool

	for _, website := range websites {
		if website.SnowEnabled {
			stationName = website.SnowDeviceName
			// Find the device to get base distance
			for _, device := range cfgData.Devices {
				if device.Name == stationName {
					baseDistance = float64(device.BaseSnowDistance)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	// If no snow-enabled website found, return nil (not an error, just disabled)
	if !found {
		logger.Debug("No snow-enabled website found, snow cache controller will not be created")
		return nil, nil
	}

	// Create calculator with logger
	calc := snow.NewCalculator(db, logger, stationName, baseDistance)

	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		db:             db,
		configProvider: configProvider,
		logger:         logger,
		calculator:     calc,
		stationName:    stationName,
		baseDistance:   baseDistance,
		stopChan:       make(chan struct{}),
	}

	return ctrl, nil
}

// Start begins the snow cache refresh loop
// This method blocks until the context is cancelled or Stop is called
func (c *Controller) Start() error {
	// Wait for snow device to start recording data
	c.logger.Infof("Snow cache refresh job waiting for data from station '%s'", c.stationName)
	dataAvailable := false
	for !dataAvailable {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Snow cache refresh job stopped before data became available")
			return nil
		case <-c.stopChan:
			c.logger.Info("Snow cache refresh job stopped before data became available")
			return nil
		case <-time.After(30 * time.Second):
			// Check if we have recent snow data (within last 24 hours)
			if c.hasRecentSnowData() {
				dataAvailable = true
				c.logger.Infof("Snow data available - starting cache refresh job for station '%s' (base_distance=%.2f)", c.stationName, c.baseDistance)
			}
		}
	}

	// Create ticker for 30-second refresh interval (snow depth totals)
	c.ticker = time.NewTicker(30 * time.Second)
	defer c.ticker.Stop()

	// Create ticker for 15-minute event caching (accumulation events for visualization)
	eventTicker := time.NewTicker(15 * time.Minute)
	defer eventTicker.Stop()

	// Do initial event caching immediately
	c.logger.Info("Running initial snow event caching...")
	if err := c.calculator.CacheEventsForTimeRanges(c.ctx); err != nil {
		c.logger.Errorf("Initial snow event caching failed: %v", err)
		// Continue despite error
	}

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Snow cache refresh job stopped (context cancelled)")
			return nil
		case <-c.stopChan:
			c.logger.Info("Snow cache refresh job stopped (stop requested)")
			return nil
		case <-c.ticker.C:
			// Check if snow is still enabled (configuration may have been reloaded)
			if !c.isSnowEnabled() {
				c.logger.Infof("Snow disabled for station '%s' - stopping cache refresh job", c.stationName)
				return nil
			}

			if err := c.calculator.RefreshCache(c.ctx); err != nil {
				c.logger.Errorf("Snow cache refresh failed: %v", err)
				// Continue running despite errors
			}
		case <-eventTicker.C:
			// Cache snow events every 15 minutes
			if !c.isSnowEnabled() {
				c.logger.Infof("Snow disabled for station '%s' - stopping cache refresh job", c.stationName)
				return nil
			}

			c.logger.Debug("Caching snow accumulation events...")
			if err := c.calculator.CacheEventsForTimeRanges(c.ctx); err != nil {
				c.logger.Errorf("Snow event caching failed: %v", err)
				// Continue running despite errors
			}
		}
	}
}

// Stop gracefully stops the controller
func (c *Controller) Stop() error {
	c.logger.Info("Stopping snow cache controller...")
	close(c.stopChan)
	if c.ticker != nil {
		c.ticker.Stop()
	}
	return nil
}

// hasRecentSnowData checks if the snow device has recorded data within the last 24 hours
func (c *Controller) hasRecentSnowData() bool {
	var count int
	query := `
		SELECT COUNT(*)
		FROM weather_5m
		WHERE stationname = $1
		  AND snowdistance IS NOT NULL
		  AND bucket >= NOW() - INTERVAL '24 hours'
		LIMIT 1
	`
	err := c.db.QueryRow(query, c.stationName).Scan(&count)
	if err != nil {
		c.logger.Debugf("Error checking for snow data: %v", err)
		return false
	}
	return count > 0
}

// isSnowEnabled checks if snow is currently enabled for the given station
// This handles configuration reloads by re-checking the configuration
func (c *Controller) isSnowEnabled() bool {
	websites, err := c.configProvider.GetWeatherWebsites()
	if err != nil {
		c.logger.Debugf("Error checking snow enabled status: %v", err)
		return false
	}

	for _, website := range websites {
		if website.SnowEnabled && website.SnowDeviceName == c.stationName {
			return true
		}
	}

	return false
}
