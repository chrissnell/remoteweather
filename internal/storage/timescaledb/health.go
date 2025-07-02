package timescaledb

import (
	"context"
	"fmt"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// startHealthMonitor starts a goroutine that periodically updates the health status
func (t *Storage) startHealthMonitor(ctx context.Context, configProvider config.ConfigProvider) {
	go func() {
		// Run initial health check immediately
		t.updateHealthStatus(configProvider)

		ticker := time.NewTicker(60 * time.Second) // Update health every minute
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.updateHealthStatus(configProvider)
			case <-ctx.Done():
				log.Info("stopping TimescaleDB health monitor")
				return
			}
		}
	}()
}

// updateHealthStatus performs a health check and updates the status in the config database
func (t *Storage) updateHealthStatus(configProvider config.ConfigProvider) {
	health := &config.StorageHealthData{
		LastCheck: time.Now(),
		Status:    "healthy",
		Message:   "TimescaleDB connection active",
	}

	// Perform health check
	if t.TimescaleDBConn == nil {
		health.Status = "unhealthy"
		health.Message = "No database connection"
		health.Error = "TimescaleDB connection is nil"
	} else {
		// Test database connection
		sqlDB, err := t.TimescaleDBConn.DB()
		if err != nil {
			health.Status = "unhealthy"
			health.Message = "Failed to get underlying database connection"
			health.Error = err.Error()
		} else if err := sqlDB.Ping(); err != nil {
			health.Status = "unhealthy"
			health.Message = "Database ping failed"
			health.Error = err.Error()
		} else {
			// Additional check: try a simple query
			var result int
			err = t.TimescaleDBConn.Raw("SELECT 1").Scan(&result).Error
			if err != nil {
				health.Status = "unhealthy"
				health.Message = "Database query test failed"
				health.Error = err.Error()
			} else {
				health.Status = "healthy"
				health.Message = fmt.Sprintf("TimescaleDB operational - ping: OK, query test: OK")
			}
		}
	}

	// Update health status in configuration database
	err := configProvider.UpdateStorageHealth("timescaledb", health)
	if err != nil {
		log.Errorf("Failed to update TimescaleDB health status: %v", err)
	} else {
		log.Infof("Updated TimescaleDB health status: %s", health.Status)
	}
}
