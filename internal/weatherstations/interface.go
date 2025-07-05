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
}

// StationFactory creates weather stations based on configuration
type StationFactory interface {
	CreateStation(config config.DeviceData) (WeatherStation, error)
}
