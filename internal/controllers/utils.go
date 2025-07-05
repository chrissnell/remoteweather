package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// HTTPClientConfig holds configuration for HTTP clients
type HTTPClientConfig struct {
	Timeout time.Duration
}

// NewHTTPClient creates a standardized HTTP client with timeout
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// ValidateTimescaleDBConfig validates TimescaleDB configuration exists
func ValidateTimescaleDBConfig(configProvider config.ConfigProvider, controllerName string) error {
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading configuration: %v", err)
	}

	if cfgData.Storage.TimescaleDB == nil || cfgData.Storage.TimescaleDB.GetConnectionString() == "" {
		return fmt.Errorf("TimescaleDB storage must be configured for the %s controller to function", controllerName)
	}

	return nil
}

// SetupDatabaseConnection creates and connects to TimescaleDB database
func SetupDatabaseConnection(configProvider config.ConfigProvider, logger *zap.SugaredLogger) (*database.Client, error) {
	db := database.NewClient(configProvider, logger)

	err := db.ConnectToTimescaleDB()
	if err != nil {
		return nil, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

	return db, nil
}

// ValidateRequiredFields checks that required configuration fields are set
func ValidateRequiredFields(fields map[string]string) error {
	for fieldName, fieldValue := range fields {
		if fieldValue == "" {
			return fmt.Errorf("%s must be set", fieldName)
		}
	}
	return nil
}

// SetDefaultValue sets a default value if the current value is empty/zero
func SetDefaultValue(current, defaultValue interface{}) interface{} {
	switch v := current.(type) {
	case string:
		if v == "" {
			return defaultValue
		}
	case int:
		if v == 0 {
			return defaultValue
		}
	case float64:
		if v == 0 {
			return defaultValue
		}
	}
	return current
}

// PeriodicTask represents a periodic task configuration
type PeriodicTask struct {
	Name     string
	Interval time.Duration
	Task     func() error
}

// RunPeriodicTask runs a task periodically until context is cancelled
func RunPeriodicTask(ctx context.Context, task PeriodicTask, logger *zap.SugaredLogger) {
	logger.Infof("Starting periodic task: %s (interval: %v)", task.Name, task.Interval)

	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := task.Task(); err != nil {
				logger.Errorf("Error in periodic task %s: %v", task.Name, err)
			}
		case <-ctx.Done():
			logger.Infof("Stopping periodic task: %s", task.Name)
			return
		}
	}
}
