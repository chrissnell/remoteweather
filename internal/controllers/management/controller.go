package management

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/interfaces"
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
	app              interfaces.AppReloader // Interface to trigger app configuration reload
}

// NewController creates a new management API controller
func NewController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, mc config.ManagementAPIData, logger *zap.SugaredLogger, app interfaces.AppReloader) (*Controller, error) {
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

	// Use existing token from database or generate a new one if none exists
	if mc.AuthToken != "" {
		ctrl.managementConfig.AuthToken = mc.AuthToken
		logger.Info("═══════════════════════════════════════════════════════════════")
		logger.Info("                MANAGEMENT API ACCESS TOKEN                    ")
		logger.Info("═══════════════════════════════════════════════════════════════")
		logger.Infof("   Token: %s", mc.AuthToken)
		logger.Info("   Use this token for API authentication")
		logger.Info("═══════════════════════════════════════════════════════════════")
	} else {
		// No token in database, generate a new one and save it
		newToken := generateAuthToken()
		ctrl.managementConfig.AuthToken = newToken

		// Save the new token to the database
		err := configProvider.UpdateController("management", &config.ControllerData{
			Type: "management",
			ManagementAPI: &config.ManagementAPIData{
				Cert:       mc.Cert,
				Key:        mc.Key,
				Port:       mc.Port,
				ListenAddr: mc.ListenAddr,
				AuthToken:  newToken,
			},
		})
		if err != nil {
			logger.Errorf("Failed to save auth token to database: %v", err)
		}

		logger.Info("═══════════════════════════════════════════════════════════════")
		logger.Info("        NEW MANAGEMENT API ACCESS TOKEN GENERATED             ")
		logger.Info("═══════════════════════════════════════════════════════════════")
		logger.Infof("   Token: %s", newToken)
		logger.Info("   *** SAVE THIS TOKEN - IT WILL NOT CHANGE ON RESTART ***")
		logger.Info("   Use this token for API authentication")
		logger.Info("═══════════════════════════════════════════════════════════════")
	}

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
	router.Use(c.corsMiddleware) // CORS is always enabled

	// Management interface routes (no auth required for UI assets)
	c.setupManagementInterface(router)

	// Authentication routes (no auth required)
	router.HandleFunc("/login", c.handlers.Login).Methods("POST")
	router.HandleFunc("/logout", c.handlers.Logout).Methods("POST")
	router.HandleFunc("/auth/status", c.handlers.GetAuthStatus).Methods("GET")

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

	// Controller management endpoints
	api.HandleFunc("/config/controllers", c.handlers.GetControllers).Methods("GET")
	api.HandleFunc("/config/controllers", c.handlers.CreateController).Methods("POST")
	api.HandleFunc("/config/controllers/{type}", c.handlers.GetController).Methods("GET")
	api.HandleFunc("/config/controllers/{type}", c.handlers.UpdateController).Methods("PUT")
	api.HandleFunc("/config/controllers/{type}", c.handlers.DeleteController).Methods("DELETE")

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
	api.HandleFunc("/test/storage", c.handlers.TestStorageConnectivity).Methods("GET")

	// Storage health monitoring endpoints
	api.HandleFunc("/health/storage", c.handlers.GetStorageHealthStatus).Methods("GET")
	api.HandleFunc("/health/storage/{type}", c.handlers.GetSingleStorageHealth).Methods("GET")

	// Utilities endpoints
	api.HandleFunc("/utils/change-token", c.handlers.ChangeAdminToken).Methods("POST")

	// WebSocket endpoint for real-time logs (requires authentication)
	api.HandleFunc("/logs/ws", c.handlers.LogsWebSocket).Methods("GET")

	// REST endpoint for logs (requires authentication)
	api.HandleFunc("/logs", c.handlers.GetLogs).Methods("GET")

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

	// Serve tab-specific URLs (all serve the same interface, JavaScript handles routing)
	router.HandleFunc("/weather-stations", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/controllers", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/storage", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/websites", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/logs", c.serveManagementInterface).Methods("GET")
	router.HandleFunc("/utilities", c.serveManagementInterface).Methods("GET")
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

// loggingMiddleware logs all requests except for noisy endpoints
func (c *Controller) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)

		// Don't log requests to /api/logs to avoid cluttering the log viewer
		if r.RequestURI != "/api/logs" {
			c.logger.Infof("%s %s %s %v", r.Method, r.RequestURI, r.RemoteAddr, time.Since(start))
		}
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

// authMiddleware validates the bearer token or session cookie
func (c *Controller) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.logger.Debugf("Auth check for %s", r.URL.Path)
		c.logger.Debugf("All cookies: %v", r.Header.Get("Cookie"))

		// Check for Bearer token first
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			expectedAuth := "Bearer " + c.managementConfig.AuthToken
			if authHeader == expectedAuth {
				c.logger.Debugf("Auth successful via Bearer token")
				next.ServeHTTP(w, r)
				return
			}
			c.logger.Debugf("Bearer token mismatch: got %s, expected %s", authHeader, expectedAuth)
		}

		// Check for session cookie
		cookie, err := r.Cookie("rw_session")
		if err == nil {
			if cookie.Value == c.managementConfig.AuthToken {
				c.logger.Debugf("Auth successful via session cookie")
				next.ServeHTTP(w, r)
				return
			}
			c.logger.Debugf("Session cookie mismatch: got %s, expected %s", cookie.Value, c.managementConfig.AuthToken)
		} else {
			c.logger.Debugf("No session cookie found: %v", err)
		}

		// Neither authentication method worked
		c.logger.Debugf("Auth failed for %s - no valid token or cookie", r.URL.Path)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
	})
}
