// Package weatherstations provides interfaces and implementations for various weather station types.
package weatherstations

import (
	"github.com/chrissnell/remoteweather/pkg/config"
)

// WeatherStation is an interface that provides standard methods for various
// weather station backends
type WeatherStation interface {
	StartWeatherStation() error
	StopWeatherStation() error
	StationName() string
	Capabilities() Capabilities
}

// StationFactory creates weather stations based on configuration
type StationFactory interface {
	CreateStation(config config.DeviceData) (WeatherStation, error)
}
