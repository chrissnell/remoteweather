// Package interfaces defines common interface types used across the application.
package interfaces

import (
	"context"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// AppReloader interface for triggering application configuration reloads and dynamic management
type AppReloader interface {
	ReloadConfiguration(ctx context.Context) error
	AddController(controllerConfig *config.ControllerData) error
	RemoveController(controllerType string) error
	AddWeatherStation(deviceName string) error
	RemoveWeatherStation(deviceName string) error
}
