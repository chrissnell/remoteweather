package weatherstations

import (
	"github.com/chrissnell/remoteweather/internal/types"
)

// WeatherStation is an interface that provides standard methods for various
// weather station backends
type WeatherStation interface {
	StartWeatherStation() error
	StationName() string
}

// StationFactory creates weather stations based on configuration
type StationFactory interface {
	CreateStation(config types.DeviceConfig) (WeatherStation, error)
}
