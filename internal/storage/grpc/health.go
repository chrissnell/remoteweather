package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// startHealthMonitor starts a goroutine that periodically updates the health status
func (g *Storage) startHealthMonitor(ctx context.Context, configProvider config.ConfigProvider) {
	go func() {
		// Run initial health check immediately
		g.updateHealthStatus(configProvider)

		ticker := time.NewTicker(60 * time.Second) // Update health every minute
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				g.updateHealthStatus(configProvider)
			case <-ctx.Done():
				log.Info("stopping gRPC health monitor")
				return
			}
		}
	}()
}

// updateHealthStatus performs a health check and updates the status in the config database
func (g *Storage) updateHealthStatus(configProvider config.ConfigProvider) {
	health := &config.StorageHealthData{
		LastCheck: time.Now(),
		Status:    "healthy",
		Message:   "gRPC server operational",
	}

	var healthDetails []string

	// Check if server is running
	if g.Server == nil {
		health.Status = "unhealthy"
		health.Message = "gRPC server not initialized"
		health.Error = "Server instance is nil"
	} else {
		healthDetails = append(healthDetails, "server: running")

		// Test if we can listen on the configured port (basic connectivity check)
		if g.GRPCConfig != nil {
			listenAddr := fmt.Sprintf(":%v", g.GRPCConfig.Port)

			// Try to create a test listener to verify port availability
			testListener, err := net.Listen("tcp", listenAddr)
			if err != nil {
				// Port might be in use by our own server, which is expected
				// Just log this as a note rather than an error
				healthDetails = append(healthDetails, fmt.Sprintf("port %d: in use (expected)", g.GRPCConfig.Port))
			} else {
				// Port is available, close test listener immediately
				testListener.Close()
				healthDetails = append(healthDetails, fmt.Sprintf("port %d: available", g.GRPCConfig.Port))
			}
		}

		// Check database connection if enabled
		if g.DBEnabled && g.DBClient != nil {
			if g.DBClient.DB == nil {
				health.Status = "unhealthy"
				health.Message = "Database client not connected"
				health.Error = "DB client connection is nil"
			} else {
				// Test database connection
				sqlDB, err := g.DBClient.DB.DB()
				if err != nil {
					health.Status = "unhealthy"
					health.Message = "Failed to get underlying database connection"
					health.Error = err.Error()
				} else if err := sqlDB.Ping(); err != nil {
					health.Status = "unhealthy"
					health.Message = "Database ping failed"
					health.Error = err.Error()
				} else {
					healthDetails = append(healthDetails, "database: connected")
				}
			}
		} else {
			healthDetails = append(healthDetails, "database: disabled")
		}

		// Check client connections
		g.ClientChanMutex.RLock()
		clientCount := len(g.ClientChans)
		g.ClientChanMutex.RUnlock()
		healthDetails = append(healthDetails, fmt.Sprintf("clients: %d connected", clientCount))

		if health.Status == "healthy" {
			health.Message = fmt.Sprintf("gRPC server operational (%s)",
				joinHealthDetails(healthDetails))
		}
	}

	// Update health status in configuration database
	err := configProvider.UpdateStorageHealth("grpc", health)
	if err != nil {
		log.Errorf("Failed to update gRPC health status: %v", err)
	} else {
		log.Infof("Updated gRPC health status: %s", health.Status)
	}
}

// joinHealthDetails joins health detail strings with commas
func joinHealthDetails(details []string) string {
	if len(details) == 0 {
		return ""
	}
	if len(details) == 1 {
		return details[0]
	}

	result := details[0]
	for i := 1; i < len(details); i++ {
		result += ", " + details[i]
	}
	return result
}
