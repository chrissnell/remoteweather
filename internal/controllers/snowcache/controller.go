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

// Controller manages the snow cache refresh lifecycle for all snow stations
type Controller struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	db             *sql.DB
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	calculators    map[string]*snow.Calculator // station name -> calculator
	ticker         *time.Ticker
	stopChan       chan struct{}
}

// NewController creates a new snow cache controller for all snowgauge devices
// Returns nil if no snow stations are configured
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

	// Load configuration to find all snowgauge devices
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Find all snowgauge devices and create calculators
	calculators := make(map[string]*snow.Calculator)

	for _, device := range cfgData.Devices {
		if device.Type == "snowgauge" && device.Enabled {
			baseDistance := float64(device.BaseSnowDistance)
			calc := snow.NewCalculator(db, logger, device.Name, baseDistance, snow.ComputerTypePELT)
			calculators[device.Name] = calc
			logger.Infof("Added snow cache calculator for station '%s' (base_distance=%.2fmm)", device.Name, baseDistance)
		}
	}

	// If no snowgauge devices found, return nil (not an error, just disabled)
	if len(calculators) == 0 {
		logger.Debug("No snowgauge devices found, snow cache controller will not be created")
		return nil, nil
	}

	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		db:             db,
		configProvider: configProvider,
		logger:         logger,
		calculators:    calculators,
		stopChan:       make(chan struct{}),
	}

	logger.Infof("Snow cache controller initialized with %d station(s)", len(calculators))
	return ctrl, nil
}

// Start begins the snow cache refresh loop for all snow stations
// This method blocks until the context is cancelled or Stop is called
func (c *Controller) Start() error {
	// Wait for snow devices to start recording data
	c.logger.Info("Snow cache refresh job waiting for data from snow stations...")
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
			// Check if we have recent snow data from any station
			if c.hasRecentSnowData() {
				dataAvailable = true
				c.logger.Infof("Snow data available - starting cache refresh job for %d station(s)", len(c.calculators))
			}
		}
	}

	// Create ticker for 30-second refresh interval (snow depth totals)
	c.ticker = time.NewTicker(30 * time.Second)
	defer c.ticker.Stop()

	// Create ticker for 15-minute event caching (accumulation events for visualization)
	eventTicker := time.NewTicker(15 * time.Minute)
	defer eventTicker.Stop()

	// Do initial event caching immediately for all stations
	c.logger.Info("Running initial snow event caching for all stations...")
	for stationName, calc := range c.calculators {
		if err := calc.CacheEventsForTimeRanges(c.ctx); err != nil {
			c.logger.Errorf("Initial snow event caching failed for station '%s': %v", stationName, err)
			// Continue despite error
		}
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
			// Refresh cache for all stations
			for stationName, calc := range c.calculators {
				if err := calc.RefreshCache(c.ctx); err != nil {
					c.logger.Errorf("Snow cache refresh failed for station '%s': %v", stationName, err)
					// Continue running despite errors
				}
			}
		case <-eventTicker.C:
			// Cache snow events every 15 minutes for all stations
			c.logger.Debug("Caching snow accumulation events for all stations...")
			for stationName, calc := range c.calculators {
				if err := calc.CacheEventsForTimeRanges(c.ctx); err != nil {
					c.logger.Errorf("Snow event caching failed for station '%s': %v", stationName, err)
					// Continue running despite errors
				}
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

// hasRecentSnowData checks if any snow device has recorded data within the last 24 hours
func (c *Controller) hasRecentSnowData() bool {
	// Check if ANY of our snow stations have recent data
	for stationName := range c.calculators {
		var count int
		query := `
			SELECT COUNT(*)
			FROM weather_5m
			WHERE stationname = $1
			  AND snowdistance IS NOT NULL
			  AND bucket >= NOW() - INTERVAL '24 hours'
			LIMIT 1
		`
		err := c.db.QueryRow(query, stationName).Scan(&count)
		if err != nil {
			c.logger.Debugf("Error checking for snow data from station '%s': %v", stationName, err)
			continue
		}
		if count > 0 {
			return true // At least one station has data
		}
	}
	return false
}
