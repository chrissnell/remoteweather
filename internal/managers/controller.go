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
	"github.com/chrissnell/remoteweather/pkg/config"
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
func NewControllerManager(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, logger *zap.SugaredLogger) (ControllerManager, error) {
	cm := &controllerManager{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		logger:         logger,
		controllers:    make([]Controller, 0),
	}

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	// Create controllers based on configuration
	for _, con := range cfgData.Controllers {
		controller, err := cm.createController(con)
		if err != nil {
			return nil, fmt.Errorf("error creating controller: %v", err)
		}
		cm.controllers = append(cm.controllers, controller)
	}

	return cm, nil
}

type controllerManager struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	controllers    []Controller
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
func (cm *controllerManager) createController(cc config.ControllerData) (Controller, error) {
	switch cc.Type {
	case "aerisweather":
		if cc.AerisWeather == nil {
			return nil, fmt.Errorf("aerisweather controller config is nil")
		}
		return aerisweather.NewAerisWeatherController(cm.ctx, cm.wg, cm.configProvider, *cc.AerisWeather, cm.logger)
	case "pwsweather":
		if cc.PWSWeather == nil {
			return nil, fmt.Errorf("pwsweather controller config is nil")
		}
		return pwsweather.NewPWSWeatherController(cm.ctx, cm.wg, cm.configProvider, *cc.PWSWeather, cm.logger)
	case "wunderground", "weatherunderground":
		if cc.WeatherUnderground == nil {
			return nil, fmt.Errorf("weatherunderground controller config is nil")
		}
		return wunderground.NewWeatherUndergroundController(cm.ctx, cm.wg, cm.configProvider, *cc.WeatherUnderground, cm.logger)
	case "restserver", "rest":
		if cc.RESTServer == nil {
			return nil, fmt.Errorf("restserver controller config is nil")
		}
		return restserver.NewController(cm.ctx, cm.wg, cm.configProvider, *cc.RESTServer, cm.logger)
	case "management":
		if cc.ManagementAPI == nil {
			return nil, fmt.Errorf("management controller config is nil")
		}
		return management.NewController(cm.ctx, cm.wg, cm.configProvider, *cc.ManagementAPI, cm.logger)
	default:
		return nil, fmt.Errorf("unknown controller type: %s", cc.Type)
	}
}
