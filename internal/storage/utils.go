package storage

import (
	"context"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// HealthChecker defines the interface for storage backends to implement health checks
type HealthChecker interface {
	CheckHealth(configProvider config.ConfigProvider) *config.StorageHealthData
}

// StartHealthMonitor starts a generic health monitoring goroutine for any storage backend
func StartHealthMonitor(ctx context.Context, configProvider config.ConfigProvider, storageType string, checker HealthChecker, interval time.Duration) {
	go func() {
		updateHealth := func() {
			health := checker.CheckHealth(configProvider)
			if err := configProvider.UpdateStorageHealth(storageType, health); err != nil {
				log.Errorf("Failed to update %s health status: %v", storageType, err)
			} else {
				log.Debugf("Updated %s health status: %s", storageType, health.Status)
			}
		}

		updateHealth()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateHealth()
			case <-ctx.Done():
				log.Infof("stopping %s health monitor", storageType)
				return
			}
		}
	}()
}

// ProcessReadings provides a standard pattern for processing readings from a channel
func ProcessReadings(ctx context.Context, wg *sync.WaitGroup, readingChan <-chan types.Reading, processor func(types.Reading) error, name string) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-readingChan:
			if err := processor(r); err != nil {
				log.Errorf("%s reading processor error: %v", name, err)
			}
		case <-ctx.Done():
			log.Infof("cancellation request received. Cancelling %s readings processor", name)
			return
		}
	}
}

// CreateHealthData creates a basic health data structure
func CreateHealthData(status, message string, err error) *config.StorageHealthData {
	health := &config.StorageHealthData{
		LastCheck: time.Now(),
		Status:    status,
		Message:   message,
	}

	if err != nil {
		health.Error = err.Error()
	}

	return health
}
