package interfaces

import "github.com/chrissnell/remoteweather/internal/weatherstations"

// WeatherStationManager defines the interface for managing weather stations
type WeatherStationManager interface {
	StartWeatherStations() error
	AddWeatherStation(deviceName string) error
	RemoveWeatherStation(deviceName string) error
	ReloadWeatherStationsConfig() error
	GetStation(deviceName string) weatherstations.WeatherStation
}
