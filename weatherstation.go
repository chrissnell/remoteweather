package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/log"
	"go.uber.org/zap"
)

// WeatherStationManager holds our active weather station backends
type WeatherStationManager struct {
	Stations []WeatherStation
}

// WeatherStation is an interface that provides standard methods for various
// weather station backends
type WeatherStation interface {
	StartWeatherStation() error
	StationName() string
}

// NewWeatherStationManager creats a WeatherStationManager object, populated with all configured
// WeatherStationEngines
func NewWeatherStationManager(ctx context.Context, wg *sync.WaitGroup, c *Config, distributor chan Reading, logger *zap.SugaredLogger) (*WeatherStationManager, error) {
	wsm := WeatherStationManager{}

	for _, s := range c.Devices {
		switch s.Type {
		case "davis":
			log.Infof("Initializing Davis weather station [%v]", s.Name)
			// Create a new DavisWeatherStation and pass the config for this station
			station, err := NewDavisWeatherStation(ctx, wg, s, distributor, logger)
			if err != nil {
				return &wsm, fmt.Errorf("error creating Davis weather station: %v", err)
			}
			wsm.Stations = append(wsm.Stations, station)
		case "campbellscientific":
			log.Infof("Initializing Campbell Scientific weather station [%v]", s.Name)
			// Create a new CampbellScientificWeatherStation and pass the config for this station
			station, err := NewCampbellScientificWeatherStation(ctx, wg, s, distributor, logger)
			if err != nil {
				return &wsm, fmt.Errorf("error creating Campbell Scientific weather station: %v", err)
			}
			wsm.Stations = append(wsm.Stations, station)
		case "snowgauge":
			log.Infof("Initializing snow gauge [%v]", s.Name)
			// Create a new SnowGaugeWeatherStation and pass the config for this station
			station, err := NewSnowGaugeWeatherStation(ctx, wg, s, distributor, logger)
			if err != nil {
				return &wsm, fmt.Errorf("error creating snow gauge: %v", err)
			}
			wsm.Stations = append(wsm.Stations, station)
		}
	}

	return &wsm, nil
}

func (wsm *WeatherStationManager) StartWeatherStations() error {
	var err error

	for _, station := range wsm.Stations {
		log.Infof("Starting weather station %v ...", station.StationName())
		err = station.StartWeatherStation()
		if err != nil {
			return err
		}
	}

	return nil
}
