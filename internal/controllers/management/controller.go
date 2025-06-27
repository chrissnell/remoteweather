package management

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Controller represents the management API controller
type Controller struct {
	ctx              context.Context
	wg               *sync.WaitGroup
	config           *types.Config
	managementConfig types.ManagementAPIConfig
	Server           http.Server
	ConfigProvider   config.ConfigProvider
	logger           *zap.SugaredLogger
	handlers         *Handlers
}

// NewController creates a new management API controller
func NewController(ctx context.Context, wg *sync.WaitGroup, c *types.Config, mc types.ManagementAPIConfig, logger *zap.SugaredLogger) (*Controller, error) {
	ctrl := &Controller{
		ctx:              ctx,
		wg:               wg,
		config:           c,
		managementConfig: mc,
		logger:           logger,
	}

	// Set default values
	if mc.Port == 0 {
		logger.Info("management API port not specified; defaulting to 8081")
		mc.Port = 8081
		ctrl.managementConfig.Port = 8081
	}

	if mc.ListenAddr == "" {
		logger.Info("management API listen-addr not provided; defaulting to 127.0.0.1 (localhost only)")
		mc.ListenAddr = "127.0.0.1"
		ctrl.managementConfig.ListenAddr = "127.0.0.1"
	}

	if mc.AuthToken == "" {
		return nil, fmt.Errorf("auth-token must be set in management API config for security")
	}

	// Initialize config provider based on what's available
	// Try common config file patterns in the current directory
	configFiles := []string{"config.yaml", "weather-station.yaml", "config.db", "weather-station.db"}

	for _, filename := range configFiles {
		if _, err := os.Stat(filename); err == nil {
			// File exists, determine type and create provider
			if strings.HasSuffix(filename, ".yaml") {
				ctrl.ConfigProvider = config.NewYAMLProvider(filename)
				logger.Infof("Management API using YAML config provider: %s", filename)
				break
			} else if strings.HasSuffix(filename, ".db") {
				sqliteProvider, err := config.NewSQLiteProvider(filename)
				if err != nil {
					logger.Warnf("Failed to initialize SQLite config provider for %s: %v", filename, err)
					continue
				}
				ctrl.ConfigProvider = sqliteProvider
				logger.Infof("Management API using SQLite config provider: %s", filename)
				break
			}
		}
	}

	if ctrl.ConfigProvider == nil {
		logger.Warn("No config provider available - configuration management will be limited")
	}

	// Create handlers
	ctrl.handlers = NewHandlers(ctrl)

	// Set up router
	router := ctrl.setupRouter()
	ctrl.Server.Addr = fmt.Sprintf("%v:%v", mc.ListenAddr, mc.Port)
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

	// API routes (with authentication)
	api := router.PathPrefix("/api").Subrouter()
	api.Use(c.authMiddleware)

	// Basic API endpoints
	api.HandleFunc("/status", c.handlers.GetStatus).Methods("GET")
	api.HandleFunc("/config", c.handlers.GetConfig).Methods("GET")

	// System discovery endpoints
	api.HandleFunc("/system/serial-ports", c.handlers.GetSerialPorts).Methods("GET")
	api.HandleFunc("/system/info", c.handlers.GetSystemInfo).Methods("GET")

	// Configuration management endpoints
	api.HandleFunc("/config/weather-stations", c.handlers.GetWeatherStations).Methods("GET")
	api.HandleFunc("/config/weather-stations", c.handlers.CreateWeatherStation).Methods("POST")
	api.HandleFunc("/config/weather-stations/{id}", c.handlers.UpdateWeatherStation).Methods("PUT")
	api.HandleFunc("/config/weather-stations/{id}", c.handlers.DeleteWeatherStation).Methods("DELETE")

	api.HandleFunc("/config/storage", c.handlers.GetStorageConfigs).Methods("GET")
	api.HandleFunc("/config/storage", c.handlers.CreateStorageConfig).Methods("POST")
	api.HandleFunc("/config/storage/{id}", c.handlers.UpdateStorageConfig).Methods("PUT")
	api.HandleFunc("/config/storage/{id}", c.handlers.DeleteStorageConfig).Methods("DELETE")

	// Testing/connectivity endpoints
	api.HandleFunc("/test/device", c.handlers.TestDeviceConnectivity).Methods("POST")
	api.HandleFunc("/test/database", c.handlers.TestDatabaseConnectivity).Methods("GET")
	api.HandleFunc("/test/serial-port", c.handlers.TestSerialPortConnectivity).Methods("GET")
	api.HandleFunc("/test/api", c.handlers.TestAPIConnectivity).Methods("POST")

	// Web interface routes (without authentication for now)
	router.HandleFunc("/", c.handlers.ServeIndex).Methods("GET")

	return router
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
