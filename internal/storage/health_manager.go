package storage

import (
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// HealthManager manages storage health status in memory
type HealthManager struct {
	mu     sync.RWMutex
	health map[string]*config.StorageHealthData
}

// GlobalHealthManager is the singleton instance for health management
var GlobalHealthManager = NewHealthManager()

// NewHealthManager creates a new health manager
func NewHealthManager() *HealthManager {
	return &HealthManager{
		health: make(map[string]*config.StorageHealthData),
	}
}

// UpdateHealth updates the health status for a storage backend
func (hm *HealthManager) UpdateHealth(storageType string, health *config.StorageHealthData) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	// Clone the health data to avoid concurrent modification
	healthCopy := &config.StorageHealthData{
		LastCheck: health.LastCheck,
		Status:    health.Status,
		Message:   health.Message,
		Error:     health.Error,
	}
	
	hm.health[storageType] = healthCopy
}

// GetHealth retrieves the health status for a specific storage backend
func (hm *HealthManager) GetHealth(storageType string) (*config.StorageHealthData, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	health, exists := hm.health[storageType]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid concurrent modification
	return &config.StorageHealthData{
		LastCheck: health.LastCheck,
		Status:    health.Status,
		Message:   health.Message,
		Error:     health.Error,
	}, true
}

// GetAllHealth retrieves all storage health statuses
func (hm *HealthManager) GetAllHealth() map[string]*config.StorageHealthData {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	// Create a copy of the map
	result := make(map[string]*config.StorageHealthData)
	for k, v := range hm.health {
		result[k] = &config.StorageHealthData{
			LastCheck: v.LastCheck,
			Status:    v.Status,
			Message:   v.Message,
			Error:     v.Error,
		}
	}
	
	return result
}

// IsHealthy checks if a storage backend is healthy
func (hm *HealthManager) IsHealthy(storageType string, maxAge time.Duration) bool {
	health, exists := hm.GetHealth(storageType)
	if !exists {
		return false
	}
	
	// Check if health data is stale
	if time.Since(health.LastCheck) > maxAge {
		return false
	}
	
	return health.Status == "healthy"
}