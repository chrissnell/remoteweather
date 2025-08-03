// Package managers provides management functionality for controllers, storage, and weather stations.
package managers

import (
	"context"
	"fmt"
	"sync"

	"github.com/chrissnell/remoteweather/internal/controllers/aerisweather"
	"github.com/chrissnell/remoteweather/internal/controllers/aprs"
	"github.com/chrissnell/remoteweather/internal/controllers/pwsweather"
	"github.com/chrissnell/remoteweather/internal/controllers/restserver"
	"github.com/chrissnell/remoteweather/internal/controllers/wunderground"
	"github.com/chrissnell/remoteweather/internal/interfaces"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// ControllerManager interface for the controller manager
type ControllerManager interface {
	StartControllers() error
	AddController(controllerConfig config.ControllerData) error
	RemoveController(controllerType string) error
	ReloadControllersConfig() error
}

// Controller is an interface that provides standard methods for various controller backends
type Controller interface {
	StartController() error
}

// NewControllerManager creates a new controller manager
func NewControllerManager(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, logger *zap.SugaredLogger, appReloader interfaces.AppReloader) (ControllerManager, error) {
	cm := &controllerManager{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		logger:         logger,
		controllers:    make(map[string]Controller),
		appReloader:    appReloader,
	}

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	// Create controllers based on configuration (REST server, management API, etc.)
	for _, con := range cfgData.Controllers {
		// Skip management controllers - they are handled separately in the app
		if con.Type == "management" {
			continue
		}
		
		// Skip weather service controllers - they will be auto-started below
		weatherServices := map[string]bool{
			"aerisweather": true,
			"pwsweather": true,
			"weatherunderground": true,
			"aprs": true,
		}
		if weatherServices[con.Type] {
			continue
		}

		controller, err := cm.createController(con)
		if err != nil {
			return nil, fmt.Errorf("error creating controller: %v", err)
		}
		cm.controllers[con.Type] = controller
	}

	// Always start weather service controllers
	weatherServices := []string{"aerisweather", "pwsweather", "weatherunderground", "aprs"}
	
	for _, serviceType := range weatherServices {
		cm.logger.Infof("Starting %s controller", serviceType)
		
		controller, err := cm.createWeatherController(serviceType)
		if err != nil {
			cm.logger.Warnf("Failed to start %s controller: %v", serviceType, err)
			continue
		}
		cm.controllers[serviceType] = controller
	}

	return cm, nil
}

type controllerManager struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	controllers    map[string]Controller
	appReloader    interfaces.AppReloader
}

func (c *controllerManager) StartControllers() error {
	c.logger.Info("Starting controller manager...")

	for controllerType, controller := range c.controllers {
		c.logger.Infof("Starting controller: %s", controllerType)
		err := controller.StartController()
		if err != nil {
			return fmt.Errorf("error starting controller %s: %v", controllerType, err)
		}
	}

	c.logger.Infof("Started %d controllers successfully", len(c.controllers))
	return nil
}

// AddController adds a new controller dynamically
func (cm *controllerManager) AddController(controllerConfig config.ControllerData) error {
	// Skip management controllers - they are handled separately in the app
	if controllerConfig.Type == "management" {
		return fmt.Errorf("management controllers are handled separately by the app")
	}

	// Check if controller already exists
	if _, exists := cm.controllers[controllerConfig.Type]; exists {
		return fmt.Errorf("controller %s already exists", controllerConfig.Type)
	}

	controller, err := cm.createController(controllerConfig)
	if err != nil {
		return fmt.Errorf("error creating controller %s: %v", controllerConfig.Type, err)
	}

	cm.controllers[controllerConfig.Type] = controller

	// Start the controller
	if err := controller.StartController(); err != nil {
		delete(cm.controllers, controllerConfig.Type)
		return fmt.Errorf("failed to start controller %s: %v", controllerConfig.Type, err)
	}

	cm.logger.Infof("Added and started controller: %s", controllerConfig.Type)
	return nil
}

// RemoveController removes a controller dynamically
func (cm *controllerManager) RemoveController(controllerType string) error {
	// Skip management controllers - they are handled separately in the app
	if controllerType == "management" {
		return fmt.Errorf("management controllers are handled separately by the app")
	}

	controller, exists := cm.controllers[controllerType]
	if !exists {
		return fmt.Errorf("controller %s not found", controllerType)
	}

	// For now, we can't cleanly stop a controller since the interface doesn't have a Stop method
	// The context cancellation will handle cleanup when the app shuts down
	delete(cm.controllers, controllerType)

	cm.logger.Infof("Removed controller: %s (will stop on next app restart)", controllerType)
	_ = controller // Keep reference to avoid "unused variable" warning
	return nil
}

// ReloadControllersConfig reloads controller configuration dynamically
func (cm *controllerManager) ReloadControllersConfig() error {
	// Load new configuration
	cfgData, err := cm.configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading configuration: %v", err)
	}

	// Track what controllers should be active
	shouldBeActive := make(map[string]config.ControllerData)
	for _, controllerConfig := range cfgData.Controllers {
		// Skip management controllers - they are handled separately in the app
		if controllerConfig.Type == "management" {
			continue
		}
		shouldBeActive[controllerConfig.Type] = controllerConfig
	}

	// Remove controllers that should no longer be active
	for controllerType := range cm.controllers {
		if _, exists := shouldBeActive[controllerType]; !exists {
			if err := cm.RemoveController(controllerType); err != nil {
				cm.logger.Errorf("Failed to remove controller %s: %v", controllerType, err)
			}
		}
	}

	// Add controllers that should be active but aren't
	for controllerType, controllerConfig := range shouldBeActive {
		if _, exists := cm.controllers[controllerType]; !exists {
			if err := cm.AddController(controllerConfig); err != nil {
				cm.logger.Errorf("Failed to add controller %s: %v", controllerType, err)
			}
		}
	}

	return nil
}

// ReloadWebsiteConfiguration reloads website configuration for the REST controller
func (cm *controllerManager) ReloadWebsiteConfiguration() error {
	// Find the REST controller
	restController, exists := cm.controllers["rest"]
	if !exists {
		return fmt.Errorf("REST controller not found")
	}

	// Type assert to get the concrete REST controller type
	if restCtrl, ok := restController.(*restserver.Controller); ok {
		return restCtrl.ReloadWebsiteConfiguration()
	}

	return fmt.Errorf("REST controller is not of the expected type")
}

// createWeatherController creates a weather service controller without configuration
func (cm *controllerManager) createWeatherController(controllerType string) (Controller, error) {
	switch controllerType {
	case "aerisweather":
		return aerisweather.NewAerisWeatherController(cm.ctx, cm.wg, cm.configProvider, cm.logger)
	case "pwsweather":
		return pwsweather.NewPWSWeatherController(cm.ctx, cm.wg, cm.configProvider, cm.logger)
	case "weatherunderground":
		return wunderground.NewWeatherUndergroundController(cm.ctx, cm.wg, cm.configProvider, cm.logger)
	case "aprs":
		return aprs.New(cm.configProvider)
	default:
		return nil, fmt.Errorf("unknown weather controller type: %s", controllerType)
	}
}

// createController creates a controller based on the controller configuration
func (cm *controllerManager) createController(cc config.ControllerData) (Controller, error) {
	switch cc.Type {
	case "restserver", "rest":
		if cc.RESTServer == nil {
			return nil, fmt.Errorf("restserver controller config is nil")
		}
		return restserver.NewController(cm.ctx, cm.wg, cm.configProvider, *cc.RESTServer, cm.logger)
	default:
		return nil, fmt.Errorf("unknown controller type: %s", cc.Type)
	}
}
