package restserver

import (
	"context"
	"embed"
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

var (
	//go:embed all:assets
	content embed.FS
)

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
		if website.DeviceID != "" {
			// Find the device by name from the website's device_id field
			for _, device := range ctrl.Devices {
				if device.Name == website.DeviceID {
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

	// Validate that all websites have at least one associated device
	for _, website := range websites {
		devices := ctrl.DevicesByWebsite[website.ID]
		if len(devices) == 0 {
			return nil, fmt.Errorf("no device is associated with website '%s' (ID: %d) - please associate a device with this website", website.Name, website.ID)
		}
	}

	// If a TimescaleDB database was configured, set up a GORM DB handle so that the
	// handlers can retrieve data
	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.ConnectionString != "" {
		var err error
		ctrl.DB, err = database.CreateConnection(cfgData.Storage.TimescaleDB.ConnectionString)
		if err != nil {
			return nil, fmt.Errorf("REST server could not connect to database: %v", err)
		}
		ctrl.DBEnabled = true
	}

	// Create handlers
	ctrl.handlers = NewHandlers(ctrl)

	// Set up embedded filesystem for assets
	assetsFS, _ := fs.Sub(fs.FS(content), "assets")
	ctrl.FS = &assetsFS

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
	router.HandleFunc("/", c.handlers.ServeIndexTemplate)
	router.HandleFunc("/js/remoteweather.js", c.handlers.ServeJS)

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
		ctx := context.WithValue(r.Context(), "website", website)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
