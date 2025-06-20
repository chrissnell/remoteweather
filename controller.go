package main

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Controllers are components that run in a loop in the background and do something.  They might
// fetch data from an external source (e.g. a weather radar image from a commercial weather service)
// or send some periodic data to an outside service (e.g. PWS Weather).  They are different from
// weatherstations and storage backends in that they do not directly generate or process real-time
// weather readings.

// ControllerManager holds our active controller backends.
type ControllerManager struct {
	Controllers []Controller
}

// Controller is an interface that provides standard methods for various controller backends
type Controller interface {
	StartController() error
}

// NewControllerManager creats a ControllerManager object, populated with all configured
// controllers
func NewControllerManager(ctx context.Context, wg *sync.WaitGroup, c *Config, logger *zap.SugaredLogger) (*ControllerManager, error) {
	cm := ControllerManager{}
	for _, con := range c.Controllers {
		switch con.Type {
		case "pwsweather":
			log.Info("Creating PWS Weather controller...")
			controller, err := NewPWSWeatherController(ctx, wg, c, con.PWSWeather, logger)
			if err != nil {
				return &ControllerManager{}, fmt.Errorf("error creating new PWS Weather controller: %v", err)
			}
			cm.Controllers = append(cm.Controllers, controller)
		case "weatherunderground":
			log.Info("Creating Weather Underground controller...")
			controller, err := NewWeatherUndergroundController(ctx, wg, c, con.WeatherUnderground, logger)
			if err != nil {
				return &ControllerManager{}, fmt.Errorf("error creating new PWS Weather controller: %v", err)
			}
			cm.Controllers = append(cm.Controllers, controller)
		case "aerisweather":
			log.Info("Creating Aeris Weather controller...")
			controller, err := NewAerisWeatherController(ctx, wg, c, con.AerisWeather, logger)
			if err != nil {
				return &ControllerManager{}, fmt.Errorf("error creating new Aeris Weather controller: %v", err)
			}
			cm.Controllers = append(cm.Controllers, controller)
		case "rest":
			log.Info("Creating REST server controller...")
			controller, err := NewRESTServerController(ctx, wg, c, con.RESTServer, logger)
			if err != nil {
				return &ControllerManager{}, fmt.Errorf("error creating new REST server controller: %v", err)
			}
			cm.Controllers = append(cm.Controllers, controller)
		}
	}

	return &cm, nil
}

func (cm *ControllerManager) StartControllers() error {
	for _, c := range cm.Controllers {
		err := c.StartController()
		if err != nil {
			return fmt.Errorf("error starting controller: %v", err)
		}
	}

	return nil
}
