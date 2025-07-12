// Package restserver provides HTTP REST API server for weather data and website hosting.
package restserver

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// websiteContextKey is the key used to store website data in request context
	websiteContextKey contextKey = "website"
)

// Assets are now handled by the GetAssets() function in assets.go

// Controller represents the REST server controller
type Controller struct {
	ctx                 context.Context
	wg                  *sync.WaitGroup
	configProvider      config.ConfigProvider
	restConfig          config.RESTServerData
	Server              http.Server
	DB                  *gorm.DB
	DBEnabled           bool
	FS                  *fs.FS
	WeatherWebsites     map[string]*config.WeatherWebsiteData // hostname -> website config
	DefaultWebsite      *config.WeatherWebsiteData            // fallback for unmatched hosts
	Devices             []config.DeviceData
	DeviceNames         map[string]bool             // device name -> exists (for fast O(1) lookups)
	DevicesByWebsite    map[int][]config.DeviceData // website_id -> devices
	AerisWeatherEnabled bool
	logger              *zap.SugaredLogger
	handlers            *Handlers
}

// NewController creates a new REST server controller
func NewController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, rc config.RESTServerData, logger *zap.SugaredLogger) (*Controller, error) {
	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		restConfig:     rc,
		logger:         logger,
	}

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	ctrl.Devices = cfgData.Devices

	// Build device name map for fast O(1) lookups
	ctrl.DeviceNames = make(map[string]bool)
	for _, device := range ctrl.Devices {
		ctrl.DeviceNames[device.Name] = true
	}

	// Load weather websites and set up hostname-based routing
	websites, err := configProvider.GetWeatherWebsites()
	if err != nil {
		return nil, fmt.Errorf("error loading weather websites: %v", err)
	}

	if len(websites) == 0 {
		return nil, fmt.Errorf("no weather websites configured - at least one website must be configured for the REST server")
	}

	// Build hostname -> website mapping and device -> website associations
	ctrl.WeatherWebsites = make(map[string]*config.WeatherWebsiteData)
	ctrl.DevicesByWebsite = make(map[int][]config.DeviceData)

	for i := range websites {
		website := &websites[i]

		// Map hostname to website (if hostname is specified)
		if website.Hostname != "" {
			ctrl.WeatherWebsites[website.Hostname] = website
		}

		// Set default website (first one or one without hostname)
		if ctrl.DefaultWebsite == nil || website.Hostname == "" {
			ctrl.DefaultWebsite = website
		}

		// Validate snow device if snow is enabled
		if website.SnowEnabled {
			if !ctrl.snowDeviceExists(website.SnowDeviceName) {
				return nil, fmt.Errorf("snow device does not exist: %s", website.SnowDeviceName)
			}
		}

		// Build device associations for this website
		var websiteDevices []config.DeviceData
		if website.DeviceID != nil {
			// Find the device by ID from the website's device_id field
			for _, device := range ctrl.Devices {
				if device.ID == *website.DeviceID {
					websiteDevices = append(websiteDevices, device)
					break // Only one device per website's device_id
				}
			}
		}
		ctrl.DevicesByWebsite[website.ID] = websiteDevices
	}

	if ctrl.DefaultWebsite == nil {
		return nil, fmt.Errorf("no default website could be determined")
	}

	// Look to see if the Aeris Weather controller has been configured.
	// If we've configured it, we will enable the /forecast endpoint later on.
	for _, con := range cfgData.Controllers {
		if con.Type == "aerisweather" {
			ctrl.AerisWeatherEnabled = true
		}
	}

	// If a DefaultListenAddr was not provided, listen on all interfaces
	if rc.DefaultListenAddr == "" {
		logger.Info("rest.default_listen_addr not provided; defaulting to 0.0.0.0 (all interfaces)")
		rc.DefaultListenAddr = "0.0.0.0"
	}

	// Set default HTTP port if not specified
	if rc.HTTPPort == 0 {
		logger.Info("rest.http_port not provided; defaulting to 8080")
		rc.HTTPPort = 8080
	}

	// Validate that all non-portal websites have at least one associated device
	for _, website := range websites {
		// Skip validation for portal websites - they don't need a specific device
		if website.IsPortal {
			continue
		}

		devices := ctrl.DevicesByWebsite[website.ID]
		if len(devices) == 0 {
			// Instead of failing, log a warning and suggest how to fix it
			deviceRef := "none"
			if website.DeviceID != nil {
				deviceRef = fmt.Sprintf("ID %d", *website.DeviceID)
			}
			logger.Warnf("Website '%s' (ID: %d) references device %s which does not exist. Available devices: %v",
				website.Name, website.ID, deviceRef, getDeviceNames(ctrl.Devices))
			logger.Warnf("To fix: Update the device_id in weather_websites table or use the management interface")
			logger.Warnf("Server will continue but this website may not function properly")
		}
	}

	// If a TimescaleDB database was configured, set up a GORM DB handle so that the
	// handlers can retrieve data
	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.GetConnectionString() != "" {
		var err error
		ctrl.DB, err = database.CreateConnection(cfgData.Storage.TimescaleDB.GetConnectionString())
		if err != nil {
			return nil, fmt.Errorf("REST server could not connect to database: %v", err)
		}
		ctrl.DBEnabled = true
	}

	// Create handlers
	ctrl.handlers = NewHandlers(ctrl)

	// Set up filesystem for assets (either from disk or embedded)
	assets := GetAssets()
	ctrl.FS = &assets

	// Set up router
	router := ctrl.setupRouter()
	ctrl.Server.Addr = fmt.Sprintf("%v:%v", rc.DefaultListenAddr, rc.HTTPPort)
	ctrl.Server.Handler = router

	return ctrl, nil
}

// StartController starts the REST server
func (c *Controller) StartController() error {
	log.Info("Starting REST server controller...")
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		if c.restConfig.TLSCertPath != "" && c.restConfig.TLSKeyPath != "" {
			if err := c.Server.ListenAndServeTLS(c.restConfig.TLSCertPath, c.restConfig.TLSKeyPath); err != http.ErrServerClosed {
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

	// Add middleware to identify the website based on Host header
	router.Use(c.websiteMiddleware)

	// API endpoints - these work for all websites
	router.HandleFunc("/span/{span}", c.handlers.GetWeatherSpan)
	router.HandleFunc("/latest", c.handlers.GetWeatherLatest)
	router.HandleFunc("/snow", c.handlers.GetSnowLatest) // Will check if snow is enabled per request

	// We only enable the /forecast endpoint if Aeris Weather has been configured.
	if c.AerisWeatherEnabled {
		router.HandleFunc("/forecast/{span}", c.handlers.GetForecast)
	}

	// Template endpoints
	router.HandleFunc("/", c.handlers.ServeWeatherWebsiteTemplate)
	router.HandleFunc("/js/weather-app.js", c.handlers.ServeWeatherAppJS)

	// Portal endpoints
	router.HandleFunc("/portal", c.handlers.ServePortal)

	// Station API endpoints
	router.HandleFunc("/api/stations", c.handlers.GetStations)
	router.HandleFunc("/api/portal-js", c.handlers.GetPortalJS)

	// Static file serving
	router.PathPrefix("/").Handler(http.FileServer(http.FS(*c.FS)))

	return router
}

// websiteMiddleware identifies the website based on Host header and adds it to the request context
func (c *Controller) websiteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract hostname from request
		host := r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}

		// Remove port from host if present
		if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
			host = host[:colonIndex]
		}

		// Find matching website
		var website *config.WeatherWebsiteData
		if mappedWebsite, exists := c.WeatherWebsites[host]; exists {
			website = mappedWebsite
		} else {
			// Use default website if no match found
			website = c.DefaultWebsite
		}

		// Add website to request context
		ctx := context.WithValue(r.Context(), websiteContextKey, website)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RefreshDeviceNames rebuilds the device name map from current devices
// Should be called when device configuration changes
func (c *Controller) RefreshDeviceNames() {
	c.DeviceNames = make(map[string]bool)
	for _, device := range c.Devices {
		c.DeviceNames[device.Name] = true
	}
}

// ReloadWebsiteConfiguration reloads website configuration from the config provider
// This should be called when website configuration changes through the management API
func (c *Controller) ReloadWebsiteConfiguration() error {
	c.logger.Info("Reloading website configuration...")

	// Load fresh website configuration
	websites, err := c.configProvider.GetWeatherWebsites()
	if err != nil {
		return fmt.Errorf("error loading weather websites: %v", err)
	}

	if len(websites) == 0 {
		return fmt.Errorf("no weather websites configured - at least one website must be configured for the REST server")
	}

	// Rebuild hostname -> website mapping and device -> website associations
	c.WeatherWebsites = make(map[string]*config.WeatherWebsiteData)
	c.DevicesByWebsite = make(map[int][]config.DeviceData)
	c.DefaultWebsite = nil

	for i := range websites {
		website := &websites[i]

		// Map hostname to website (if hostname is specified)
		if website.Hostname != "" {
			c.WeatherWebsites[website.Hostname] = website
		}

		// Set default website (first one or one without hostname)
		if c.DefaultWebsite == nil || website.Hostname == "" {
			c.DefaultWebsite = website
		}

		// Validate snow device if snow is enabled
		if website.SnowEnabled {
			if !c.snowDeviceExists(website.SnowDeviceName) {
				c.logger.Warnf("Snow device '%s' for website '%s' does not exist", website.SnowDeviceName, website.Name)
			}
		}

		// Build device associations for this website
		var websiteDevices []config.DeviceData
		if website.DeviceID != nil {
			// Find the device by ID from the website's device_id field
			for _, device := range c.Devices {
				if device.ID == *website.DeviceID {
					websiteDevices = append(websiteDevices, device)
					break // Only one device per website's device_id
				}
			}
		}
		c.DevicesByWebsite[website.ID] = websiteDevices
	}

	if c.DefaultWebsite == nil {
		return fmt.Errorf("no default website could be determined")
	}

	c.logger.Info("Website configuration reloaded successfully")
	return nil
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

// getDeviceNames returns a slice of all device names
func getDeviceNames(devices []config.DeviceData) []string {
	var names []string
	for _, device := range devices {
		names = append(names, device.Name)
	}
	return names
}
