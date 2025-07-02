package management

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Controller represents the management API controller
type Controller struct {
	ctx              context.Context
	wg               *sync.WaitGroup
	configProvider   config.ConfigProvider
	managementConfig config.ManagementAPIData
	Server           http.Server
	ConfigProvider   config.ConfigProvider
	logger           *zap.SugaredLogger
	handlers         *Handlers
	app              AppReloader // Interface to trigger app configuration reload
}

// AppReloader interface for triggering application configuration reloads
type AppReloader interface {
	ReloadConfiguration(ctx context.Context) error
}

// NewController creates a new management API controller
func NewController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, mc config.ManagementAPIData, logger *zap.SugaredLogger, app AppReloader) (*Controller, error) {
	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		logger:         logger,
		app:            app,
	}

	// Use ManagementAPIData directly
	ctrl.managementConfig = mc

	// Set default values
	if ctrl.managementConfig.Port == 0 {
		logger.Info("management API port not specified; defaulting to 8081")
		ctrl.managementConfig.Port = 8081
	}

	if ctrl.managementConfig.ListenAddr == "" {
		logger.Info("management API listen-addr not provided; defaulting to 127.0.0.1 (localhost only)")
		ctrl.managementConfig.ListenAddr = "127.0.0.1"
	}

	token := generateAuthToken()
	logger.Infof("Generated management API auth token: %s", token)
	ctrl.managementConfig.AuthToken = token

	// Config provider is already available from the parameter
	ctrl.ConfigProvider = configProvider
	logger.Info("Management API using provided config provider")

	if ctrl.ConfigProvider == nil {
		logger.Warn("No config provider available - configuration management will be limited")
	}

	// Create handlers
	ctrl.handlers = NewHandlers(ctrl)

	// Set up router
	router := ctrl.setupRouter()
	ctrl.Server.Addr = fmt.Sprintf("%v:%v", ctrl.managementConfig.ListenAddr, ctrl.managementConfig.Port)
	ctrl.Server.Handler = router

	return ctrl, nil
}

// StartController starts the management API server
func (c *Controller) StartController() error {
	log.Info("Starting management API controller...")
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		c.logger.Infof("Management API server starting on %s", c.Server.Addr)

		var err error
		if c.managementConfig.Cert != "" && c.managementConfig.Key != "" {
			c.logger.Info("Starting management API server with TLS")
			err = c.Server.ListenAndServeTLS(c.managementConfig.Cert, c.managementConfig.Key)
		} else {
			c.logger.Info("Starting management API server without TLS")
			err = c.Server.ListenAndServe()
		}

		if err != http.ErrServerClosed {
			log.Errorf("Management API server error: %v", err)
		}
	}()

	go func() {
		<-c.ctx.Done()
		log.Info("Shutting down the management API server...")
		c.Server.Shutdown(context.Background())
	}()

	return nil
}

// setupRouter configures the HTTP router with all endpoints
func (c *Controller) setupRouter() *mux.Router {
	router := mux.NewRouter()

	// Apply middleware
	router.Use(c.loggingMiddleware)
	if c.managementConfig.EnableCORS {
		router.Use(c.corsMiddleware)
	}

	// Management interface routes (no auth required for UI assets)
	c.setupManagementInterface(router)

	// API routes (with authentication)
	api := router.PathPrefix("/api").Subrouter()
	api.Use(c.authMiddleware)

	// Basic API endpoints
	api.HandleFunc("/status", c.handlers.GetStatus).Methods("GET")
	api.HandleFunc("/config", c.handlers.GetConfig).Methods("GET")
	api.HandleFunc("/config/validate", c.handlers.ValidateConfig).Methods("GET")
	api.HandleFunc("/config/reload", c.handlers.ReloadConfig).Methods("POST")

	// System discovery endpoints
	api.HandleFunc("/system/serial-ports", c.handlers.GetSerialPorts).Methods("GET")
	api.HandleFunc("/system/info", c.handlers.GetSystemInfo).Methods("GET")

	// Configuration management endpoints
	api.HandleFunc("/config/weather-stations", c.handlers.GetWeatherStations).Methods("GET")
	api.HandleFunc("/config/weather-stations", c.handlers.CreateWeatherStation).Methods("POST")
	api.HandleFunc("/config/weather-stations/{id}", c.handlers.GetWeatherStation).Methods("GET")
	api.HandleFunc("/config/weather-stations/{id}", c.handlers.UpdateWeatherStation).Methods("PUT")
	api.HandleFunc("/config/weather-stations/{id}", c.handlers.DeleteWeatherStation).Methods("DELETE")

	api.HandleFunc("/config/storage", c.handlers.GetStorageConfigs).Methods("GET")
	api.HandleFunc("/config/storage", c.handlers.CreateStorageConfig).Methods("POST")
	api.HandleFunc("/config/storage/{id}", c.handlers.UpdateStorageConfig).Methods("PUT")
	api.HandleFunc("/config/storage/{id}", c.handlers.DeleteStorageConfig).Methods("DELETE")

	// Weather website management endpoints
	api.HandleFunc("/config/websites", c.handlers.GetWeatherWebsites).Methods("GET")
	api.HandleFunc("/config/websites", c.handlers.CreateWeatherWebsite).Methods("POST")
	api.HandleFunc("/config/websites/{id}", c.handlers.GetWeatherWebsite).Methods("GET")
	api.HandleFunc("/config/websites/{id}", c.handlers.UpdateWeatherWebsite).Methods("PUT")
	api.HandleFunc("/config/websites/{id}", c.handlers.DeleteWeatherWebsite).Methods("DELETE")

	// Testing/connectivity endpoints
	api.HandleFunc("/test/device", c.handlers.TestDeviceConnectivity).Methods("POST")
	api.HandleFunc("/test/database", c.handlers.TestDatabaseConnectivity).Methods("GET")
	api.HandleFunc("/test/serial-port", c.handlers.TestSerialPortConnectivity).Methods("GET")
	api.HandleFunc("/test/api", c.handlers.TestAPIConnectivity).Methods("POST")
	api.HandleFunc("/test/current-reading", c.handlers.GetCurrentWeatherReading).Methods("POST")

	return router
}

// setupManagementInterface sets up routes for the management web interface
func (c *Controller) setupManagementInterface(router *mux.Router) {
	assets := GetAssets()

	// Serve static assets
	router.PathPrefix("/css/").Handler(http.StripPrefix("/", http.FileServer(http.FS(assets))))
	router.PathPrefix("/js/").Handler(http.StripPrefix("/", http.FileServer(http.FS(assets))))
	router.PathPrefix("/images/").Handler(http.StripPrefix("/", http.FileServer(http.FS(assets))))

	// Serve the main management interface
	router.HandleFunc("/", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/management", c.serveManagementInterface).Methods("GET")
}

// serveManagementInterface serves the main management interface HTML
func (c *Controller) serveManagementInterface(w http.ResponseWriter, r *http.Request) {
	assets := GetAssets()

	// Read the index.html file content
	content, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		c.logger.Errorf("Failed to read index.html: %v", err)
		http.Error(w, "Management interface not available", http.StatusInternalServerError)
		return
	}

	// Set content type and write response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

// loggingMiddleware logs all requests
func (c *Controller) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		c.logger.Infof("%s %s %s %v", r.Method, r.RequestURI, r.RemoteAddr, time.Since(start))
	})
}

// corsMiddleware adds CORS headers
func (c *Controller) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates the bearer token
func (c *Controller) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		expectedAuth := "Bearer " + c.managementConfig.AuthToken
		if authHeader != expectedAuth {
			http.Error(w, "Invalid authorization token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
