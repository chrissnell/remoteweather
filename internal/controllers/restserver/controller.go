package restserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Controller represents the REST server controller
type Controller struct {
	ctx                 context.Context
	wg                  *sync.WaitGroup
	config              *types.Config
	restConfig          types.RESTServerConfig
	Server              http.Server
	DB                  *gorm.DB
	DBEnabled           bool
	WeatherSiteConfig   *types.WeatherSiteConfig
	Devices             []types.DeviceConfig
	AerisWeatherEnabled bool
	logger              *zap.SugaredLogger
	handlers            *Handlers
}

// NewController creates a new REST server controller
func NewController(ctx context.Context, wg *sync.WaitGroup, c *types.Config, rc types.RESTServerConfig, logger *zap.SugaredLogger) (*Controller, error) {
	ctrl := &Controller{
		ctx:        ctx,
		wg:         wg,
		config:     c,
		restConfig: rc,
		Devices:    c.Devices,
		logger:     logger,
	}

	ctrl.WeatherSiteConfig = &rc.WeatherSiteConfig

	if rc.WeatherSiteConfig.SnowEnabled {
		if !ctrl.snowDeviceExists(rc.WeatherSiteConfig.SnowDevice) {
			return nil, fmt.Errorf("snow device does not exist: %s", rc.WeatherSiteConfig.SnowDevice)
		}

		for _, d := range ctrl.Devices {
			if d.Name == rc.WeatherSiteConfig.SnowDevice {
				ctrl.WeatherSiteConfig.SnowBaseDistance = float32(d.BaseSnowDistance)
			}
		}
	}

	// Look to see if the Aeris Weather controller has been configured.
	// If we've configured it, we will enable the /forecast endpoint later on.
	for _, con := range c.Controllers {
		if con.Type == "aerisweather" {
			ctrl.AerisWeatherEnabled = true
		}
	}

	// If a ListenAddr was not provided, listen on all interfaces
	if rc.ListenAddr == "" {
		logger.Info("rest.listen_addr not provided; defaulting to 0.0.0.0 (all interfaces)")
		rc.ListenAddr = "0.0.0.0"
	}

	if rc.WeatherSiteConfig.PullFromDevice == "" {
		return nil, fmt.Errorf("pull-from-device must be set in weather-site config")
	} else {
		if !ctrl.validatePullFromStation(rc.WeatherSiteConfig.PullFromDevice) {
			return nil, fmt.Errorf("pull-from-device %v is not a valid station name", rc.WeatherSiteConfig.PullFromDevice)
		}
	}

	// If a TimescaleDB database was configured, set up a GORM DB handle so that the
	// handlers can retrieve data
	if c.Storage.TimescaleDB.ConnectionString != "" {
		var err error
		ctrl.DB, err = database.CreateConnection(c.Storage.TimescaleDB.ConnectionString)
		if err != nil {
			return nil, fmt.Errorf("REST server could not connect to database: %v", err)
		}
		ctrl.DBEnabled = true
	}

	// Create handlers
	ctrl.handlers = NewHandlers(ctrl)

	// Set up router
	router := ctrl.setupRouter()
	ctrl.Server.Addr = fmt.Sprintf("%v:%v", rc.ListenAddr, rc.Port)
	ctrl.Server.Handler = router

	return ctrl, nil
}

// StartController starts the REST server
func (c *Controller) StartController() error {
	log.Info("Starting REST server controller...")
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		if c.restConfig.Cert != "" && c.restConfig.Key != "" {
			if err := c.Server.ListenAndServeTLS(c.restConfig.Cert, c.restConfig.Key); err != http.ErrServerClosed {
				log.Errorf("REST server error: %v", err)
			}
		} else {
			if err := c.Server.ListenAndServe(); err != http.ErrServerClosed {
				log.Errorf("REST server error: %v", err)
			}
		}
	}()

	go func() {
		<-c.ctx.Done()
		log.Info("Shutting down the REST server...")
		c.Server.Shutdown(context.Background())
	}()

	return nil
}

// setupRouter configures the HTTP router with all endpoints
func (c *Controller) setupRouter() *mux.Router {
	router := mux.NewRouter()

	// API endpoints
	router.HandleFunc("/span/{span}", c.handlers.GetWeatherSpan)
	router.HandleFunc("/latest", c.handlers.GetWeatherLatest)

	if c.restConfig.WeatherSiteConfig.SnowEnabled {
		router.HandleFunc("/snow", c.handlers.GetSnowLatest)
	}

	// We only enable the /forecast endpoint if Aeris Weather has been configured.
	if c.AerisWeatherEnabled {
		router.HandleFunc("/forecast/{span}", c.handlers.GetForecast)
	}

	// Template endpoints
	router.HandleFunc("/", c.handlers.ServeIndexTemplate)
	router.HandleFunc("/js/remoteweather.js", c.handlers.ServeJS)

	// Static file serving (disabled for now until we handle assets properly)
	// router.PathPrefix("/").Handler(http.FileServer(http.FS(*c.FS)))

	return router
}

// validatePullFromStation validates that the station name exists in config
func (c *Controller) validatePullFromStation(pullFromDevice string) bool {
	if len(c.Devices) > 0 {
		for _, station := range c.Devices {
			if station.Name == pullFromDevice {
				return true
			}
		}
	}
	return false
}

// snowDeviceExists checks if a snow device exists in the configuration
func (c *Controller) snowDeviceExists(name string) bool {
	for _, device := range c.Devices {
		if device.Name == name && device.Type == "snowgauge" {
			return true
		}
	}
	return false
}
