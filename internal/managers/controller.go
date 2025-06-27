package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/controllers/aerisweather"
	"github.com/chrissnell/remoteweather/internal/controllers/management"
	"github.com/chrissnell/remoteweather/internal/controllers/pwsweather"
	"github.com/chrissnell/remoteweather/internal/controllers/restserver"
	"github.com/chrissnell/remoteweather/internal/controllers/wunderground"
	"github.com/chrissnell/remoteweather/internal/types"
	"go.uber.org/zap"
)

// ControllerManager interface for the controller manager
type ControllerManager interface {
	StartControllers() error
}

// Controller is an interface that provides standard methods for various controller backends
type Controller interface {
	StartController() error
}

// NewControllerManager creates a new controller manager
func NewControllerManager(ctx context.Context, wg *sync.WaitGroup, c *types.Config, logger *zap.SugaredLogger) (ControllerManager, error) {
	cm := &controllerManager{
		ctx:         ctx,
		wg:          wg,
		config:      c,
		logger:      logger,
		controllers: make([]Controller, 0),
	}

	// Create controllers based on configuration
	for _, con := range c.Controllers {
		controller, err := cm.createController(con)
		if err != nil {
			return nil, fmt.Errorf("error creating controller: %v", err)
		}
		cm.controllers = append(cm.controllers, controller)
	}

	return cm, nil
}

type controllerManager struct {
	ctx         context.Context
	wg          *sync.WaitGroup
	config      *types.Config
	logger      *zap.SugaredLogger
	controllers []Controller
}

func (c *controllerManager) StartControllers() error {
	c.logger.Info("Starting controller manager...")

	for _, controller := range c.controllers {
		err := controller.StartController()
		if err != nil {
			return fmt.Errorf("error starting controller: %v", err)
		}
	}

	c.logger.Infof("Started %d controllers successfully", len(c.controllers))
	return nil
}

// createController creates a controller based on the controller configuration
func (cm *controllerManager) createController(cc types.ControllerConfig) (Controller, error) {
	switch cc.Type {
	case "aerisweather":
		return aerisweather.NewAerisWeatherController(cm.ctx, cm.wg, cm.config, cc.AerisWeather, cm.logger)
	case "pwsweather":
		return pwsweather.NewPWSWeatherController(cm.ctx, cm.wg, cm.config, cc.PWSWeather, cm.logger)
	case "wunderground", "weatherunderground":
		return wunderground.NewWeatherUndergroundController(cm.ctx, cm.wg, cm.config, cc.WeatherUnderground, cm.logger)
	case "restserver", "rest":
		return restserver.NewController(cm.ctx, cm.wg, cm.config, cc.RESTServer, cm.logger)
	case "management":
		return management.NewController(cm.ctx, cm.wg, cm.config, cc.ManagementAPI, cm.logger)
	default:
		return nil, fmt.Errorf("unknown controller type: %s", cc.Type)
	}
}
