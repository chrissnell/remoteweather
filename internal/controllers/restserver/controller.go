// Package restserver provides HTTP REST API server for weather data and website hosting.
package restserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/grpcutil"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	weather "github.com/chrissnell/remoteweather/protocols/remoteweather"
	weatherapps "github.com/chrissnell/remoteweather/protocols/weatherapps"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// websiteContextKey is the key used to store website data in request context
	websiteContextKey contextKey = "website"
)

// Assets are now handled by the GetAssets() function in assets.go

// Controller represents the unified REST/gRPC server controller

// ErrNoReadingsFound is returned when no weather readings are available for a station
// This is used to distinguish from actual database errors
var ErrNoReadingsFound = fmt.Errorf("no recent readings available")

type Controller struct {
	ctx                 context.Context
	wg                  *sync.WaitGroup
	configProvider      config.ConfigProvider
	restConfig          config.RESTServerData
	Server              http.Server
	HTTPSServer         http.Server              // HTTPS server instance
	GRPCServer          *grpc.Server             // gRPC server instance
	DeviceManager       *grpcutil.DeviceManager  // Device manager for gRPC
	DB                  *gorm.DB
	DBEnabled           bool
	FS                  *fs.FS
	WeatherWebsites       map[string]*config.WeatherWebsiteData // hostname -> website config
	DefaultWebsite        *config.WeatherWebsiteData            // fallback for unmatched hosts
	Devices               []config.DeviceData
	DeviceNames           map[string]bool                       // device name -> exists (for fast O(1) lookups)
	DevicesByWebsite      map[int][]config.DeviceData           // website_id -> devices
	SnowBaseDistanceCache map[int]float32                       // website_id -> snow base distance
	AerisWeatherEnabled bool
	logger              *zap.SugaredLogger
	handlers            *Handlers
	tlsConfigs          map[string]*tls.Config   // hostname -> TLS config for SNI

	weather.UnimplementedWeatherV1Server
	weatherapps.UnimplementedWeatherAppsV1Server
}

// NewController creates a new unified REST/gRPC server controller
func NewController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, rc config.RESTServerData, logger *zap.SugaredLogger) (*Controller, error) {
	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		restConfig:     rc,
		logger:         logger,
		tlsConfigs:     make(map[string]*tls.Config),
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

	// Create device manager for gRPC
	ctrl.DeviceManager = grpcutil.NewDeviceManager(cfgData.Devices)

	// Always initialize gRPC server without TLS
	// When using HTTPS mode, TLS is handled by the outer TLS listener
	ctrl.GRPCServer = grpc.NewServer()

	// Register the weather services and reflection
	weather.RegisterWeatherV1Server(ctrl.GRPCServer, ctrl)       // Raw station data protocol
	weatherapps.RegisterWeatherAppsV1Server(ctrl.GRPCServer, ctrl) // End-user protocol with calculated values
	reflection.Register(ctrl.GRPCServer)

	// Load weather websites and set up hostname-based routing
	websites, err := configProvider.GetWeatherWebsites()
	if err != nil {
		return nil, fmt.Errorf("error loading weather websites: %v", err)
	}

	// Allow REST server to start without websites configured
	if len(websites) == 0 {
		ctrl.logger.Warn("No weather websites configured - REST server will start but won't serve weather data")
	}

	// Build hostname -> website mapping and device -> website associations
	ctrl.WeatherWebsites = make(map[string]*config.WeatherWebsiteData)
	ctrl.DevicesByWebsite = make(map[int][]config.DeviceData)
	ctrl.SnowBaseDistanceCache = make(map[int]float32)

	for i := range websites {
		website := &websites[i]

		// Map hostname to website (if hostname is specified)
		if website.Hostname != "" {
			ctrl.WeatherWebsites[website.Hostname] = website
			
			// Load TLS configuration if certificates are provided
			if website.TLSCertPath != "" && website.TLSKeyPath != "" {
				cert, err := tls.LoadX509KeyPair(website.TLSCertPath, website.TLSKeyPath)
				if err != nil {
					logger.Errorf("Failed to load TLS certificate for website %s: %v", website.Name, err)
					// Continue without TLS for this website
				} else {
					// Build certificate chains if intermediates are present
					if cert.Leaf == nil && len(cert.Certificate) > 0 {
						// Parse the leaf certificate
						x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
						if err == nil {
							cert.Leaf = x509Cert
						}
					}
					
					ctrl.tlsConfigs[website.Hostname] = &tls.Config{
						Certificates: []tls.Certificate{cert},
						MinVersion:   tls.VersionTLS12,
					}
					logger.Infof("Loaded TLS certificate for website %s (hostname: %s)", website.Name, website.Hostname)
				}
			}
		}

		// Set default website (first one or one without hostname)
		if ctrl.DefaultWebsite == nil || website.Hostname == "" {
			ctrl.DefaultWebsite = website
		}

		// Validate snow device if snow is enabled and cache base distance
		if website.SnowEnabled {
			found := false
			for _, device := range ctrl.Devices {
				if device.Name == website.SnowDeviceName {
					ctrl.SnowBaseDistanceCache[website.ID] = float32(device.BaseSnowDistance)
					found = true

					// Configure the snow cache refresh job with station-specific parameters
					// (Migration 013 creates the job, we configure it here)
					if ctrl.DBEnabled && ctrl.DB != nil {
						configureSnowCacheJob := fmt.Sprintf(`
							DO $$
							DECLARE
								job_record RECORD;
							BEGIN
								-- Find the snow cache refresh job
								SELECT job_id INTO job_record
								FROM timescaledb_information.jobs
								WHERE proc_name = 'refresh_snow_cache'
								LIMIT 1;

								-- Configure it with station parameters if found
								IF FOUND THEN
									PERFORM alter_job(
										job_record.job_id,
										config => jsonb_build_object(
											'stationname', '%s',
											'base_distance', %f
										)
									);
									RAISE NOTICE 'Configured snow cache refresh job for station %% with base_distance %%', '%s', %f;
								END IF;
							END $$;
						`, website.SnowDeviceName, float64(device.BaseSnowDistance), website.SnowDeviceName, float64(device.BaseSnowDistance))

						err := ctrl.DB.Exec(configureSnowCacheJob).Error
						if err != nil {
							logger.Warnf("Failed to configure snow cache refresh job for %s: %v", website.SnowDeviceName, err)
						} else {
							logger.Infof("Configured snow cache refresh job for station '%s' with base_distance=%.2f", website.SnowDeviceName, device.BaseSnowDistance)
						}
					}

					break
				}
			}
			if !found {
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

	// DefaultWebsite can be nil if no websites are configured
	if ctrl.DefaultWebsite == nil && len(websites) > 0 {
		return nil, fmt.Errorf("no default website could be determined")
	}

	// Check if any device has Aeris Weather enabled
	for _, device := range ctrl.Devices {
		if device.AerisEnabled {
			ctrl.AerisWeatherEnabled = true
			break
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
	assets, err := GetAssets()
	if err != nil {
		return nil, fmt.Errorf("failed to load assets: %v", err)
	}
	ctrl.FS = &assets

	// Set up router
	router := ctrl.setupRouter()
	ctrl.Server.Addr = fmt.Sprintf("%v:%v", rc.DefaultListenAddr, rc.HTTPPort)
	ctrl.Server.Handler = router

	return ctrl, nil
}

// StartController starts the REST and gRPC servers on separate ports
func (c *Controller) StartController() error {
	log.Info("Starting REST and gRPC servers...")

	c.wg.Add(2) // One for REST, one for gRPC

	// Start REST/HTTPS server
	go c.startRESTServer()

	// Start gRPC server
	go c.startGRPCServer()

	// Handle graceful shutdown
	go c.handleShutdown()

	return nil
}

// startRESTServer starts the REST server (HTTP or HTTPS)
func (c *Controller) startRESTServer() {
	defer c.wg.Done()

	listenAddr := fmt.Sprintf("%s:%d", c.restConfig.DefaultListenAddr, c.restConfig.HTTPPort)

	if c.shouldUseHTTPS() {
		c.startHTTPSServer(listenAddr)
	} else {
		c.startHTTPServer(listenAddr)
	}
}

// startHTTPSServer starts the HTTPS server with HTTP/2 support
func (c *Controller) startHTTPSServer(listenAddr string) {
	tlsConfig := &tls.Config{
		GetCertificate: c.getCertificate,
		NextProtos:     []string{"h2", "http/1.1"}, // Enable HTTP/2
	}

	listener, err := tls.Listen("tcp", listenAddr, tlsConfig)
	if err != nil {
		log.Errorf("Failed to create HTTPS listener: %v", err)
		return
	}
	defer listener.Close()

	// Configure HTTP/2 support - native, no cmux interference
	if err := http2.ConfigureServer(&c.Server, &http2.Server{}); err != nil {
		log.Errorf("Failed to configure HTTP/2: %v", err)
		return
	}

	c.Server.Handler = c.setupRouter()

	log.Infof("HTTPS server listening on %s (HTTP/2 enabled)", listenAddr)
	if err := c.Server.Serve(listener); err != http.ErrServerClosed {
		log.Errorf("HTTPS server error: %v", err)
	}
}

// startHTTPServer starts the HTTP server
func (c *Controller) startHTTPServer(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorf("Failed to create HTTP listener: %v", err)
		return
	}
	defer listener.Close()

	c.Server.Handler = c.setupRouter()

	log.Infof("HTTP server listening on %s", listenAddr)
	if err := c.Server.Serve(listener); err != http.ErrServerClosed {
		log.Errorf("HTTP server error: %v", err)
	}
}

// startGRPCServer starts the gRPC server on a separate port
func (c *Controller) startGRPCServer() {
	defer c.wg.Done()

	grpcAddr := fmt.Sprintf("%s:%d",
		c.restConfig.GRPCListenAddr,
		c.restConfig.GRPCPort)

	var listener net.Listener
	var err error

	if c.shouldUseGRPCTLS() {
		listener, err = c.createGRPCTLSListener(grpcAddr)
	} else {
		listener, err = net.Listen("tcp", grpcAddr)
	}

	if err != nil {
		log.Errorf("Failed to create gRPC listener: %v", err)
		return
	}
	defer listener.Close()

	tlsStatus := "without TLS"
	if c.shouldUseGRPCTLS() {
		tlsStatus = "with TLS"
	}

	log.Infof("gRPC server listening on %s (%s)", grpcAddr, tlsStatus)
	if err := c.GRPCServer.Serve(listener); err != nil {
		log.Errorf("gRPC server error: %v", err)
	}
}

// shouldUseHTTPS returns true if HTTPS should be used for the REST server
func (c *Controller) shouldUseHTTPS() bool {
	return len(c.tlsConfigs) > 0
}

// shouldUseGRPCTLS returns true if TLS should be used for the gRPC server
func (c *Controller) shouldUseGRPCTLS() bool {
	return c.restConfig.GRPCCertPath != "" && c.restConfig.GRPCKeyPath != ""
}

// createGRPCTLSListener creates a TLS listener for gRPC
func (c *Controller) createGRPCTLSListener(addr string) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(
		c.restConfig.GRPCCertPath,
		c.restConfig.GRPCKeyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load gRPC TLS cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2"}, // gRPC requires HTTP/2
	}

	return tls.Listen("tcp", addr, tlsConfig)
}

// handleShutdown handles graceful shutdown of both servers
func (c *Controller) handleShutdown() {
	<-c.ctx.Done()
	log.Info("Shutting down servers...")

	// Shutdown REST server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("REST server shutdown error: %v", err)
	}

	// Stop gRPC server gracefully
	c.GRPCServer.GracefulStop()

	log.Info("Servers shutdown complete")
}

// setupRouter configures the HTTP router with all endpoints
func (c *Controller) setupRouter() *mux.Router {
	router := mux.NewRouter()

	// Add middleware to identify the website based on Host header
	router.Use(c.websiteMiddleware)

	// Add HTTP logging middleware
	router.Use(c.httpLoggingMiddleware)

	// Add gzip compression middleware for all responses
	router.Use(handlers.CompressHandler)

	// API endpoints - these work for all websites
	router.HandleFunc("/span/{span}", c.handlers.GetWeatherSpan)
	router.HandleFunc("/latest", c.handlers.GetWeatherLatest)
	router.HandleFunc("/snow", c.handlers.GetSnowLatest) // Will check if snow is enabled per request
	router.HandleFunc("/almanac", c.handlers.GetAlmanac)

	// We only enable the /forecast endpoint if Aeris Weather has been configured.
	if c.AerisWeatherEnabled {
		router.HandleFunc("/forecast/{span}", c.handlers.GetForecast)
	}

	// Template endpoints
	router.HandleFunc("/", c.handlers.ServeWeatherWebsiteTemplate)
	router.HandleFunc("/js/weather-app.js", c.handlers.ServeWeatherAppJS)

	// Portal endpoints
	router.HandleFunc("/portal", c.handlers.ServePortal)

	// Privacy policy endpoint
	router.HandleFunc("/privacy", c.handlers.ServePrivacy)

	// Support page endpoint
	router.HandleFunc("/support", c.handlers.ServeSupport)

	// Station API endpoints
	router.HandleFunc("/api/stations", c.handlers.GetStations)
	router.HandleFunc("/api/remote-stations", c.handlers.GetRemoteStations)
	router.HandleFunc("/stationinfo", c.handlers.GetStationInfo)

	// Serve fonts with long-term caching headers
	router.PathPrefix("/fonts/").Handler(c.cachingMiddleware(365*24*time.Hour, http.FileServer(http.FS(*c.FS))))
	
	// Static file serving for other assets
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

		// If no websites are configured and request is not to localhost, return error
		if len(c.WeatherWebsites) == 0 && c.DefaultWebsite == nil {
			if host != "localhost" && host != "127.0.0.1" && host != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "No weather websites are configured",
					"message": "The REST server is running but no weather websites have been configured yet",
				})
				return
			}
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

// httpLoggingMiddleware logs all HTTP requests to the separate HTTP log buffer
func (c *Controller) httpLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Debug log to verify middleware is being hit
		c.logger.Debugf("HTTP request: %s %s", r.Method, r.URL.Path)

		// Wrap the ResponseWriter to capture status code and size
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Log the request
		duration := time.Since(start)
		website := ""
		if ws, ok := r.Context().Value(websiteContextKey).(*config.WeatherWebsiteData); ok && ws != nil {
			website = ws.Name
		}

		// Get client IP, preferring X-Forwarded-For if present
		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			if comma := strings.Index(xff, ","); comma != -1 {
				clientIP = strings.TrimSpace(xff[:comma])
			} else {
				clientIP = strings.TrimSpace(xff)
			}
		}
		
		// Strip port from IP address
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}

		// Include query string in the path if present
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path = path + "?" + r.URL.RawQuery
		}

		log.LogHTTPRequest(
			r.Method,
			path,
			wrapped.statusCode,
			duration,
			wrapped.bytesWritten,
			clientIP,
			r.UserAgent(),
			website,
			r.Header.Get("Referer"),
			nil,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// cachingMiddleware adds cache control headers for static assets
func (c *Controller) cachingMiddleware(maxAge time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set cache control headers
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", int(maxAge.Seconds())))
		
		// Add ETag support for better caching
		w.Header().Set("Vary", "Accept-Encoding")
		
		// Call the next handler
		next.ServeHTTP(w, r)
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

	// Allow REST server to operate without websites configured
	if len(websites) == 0 {
		c.logger.Warn("No weather websites configured after reload - REST server will continue but won't serve weather data")
	}

	// Rebuild hostname -> website mapping and device -> website associations
	c.WeatherWebsites = make(map[string]*config.WeatherWebsiteData)
	c.DevicesByWebsite = make(map[int][]config.DeviceData)
	c.SnowBaseDistanceCache = make(map[int]float32)
	c.tlsConfigs = make(map[string]*tls.Config)
	c.DefaultWebsite = nil

	for i := range websites {
		website := &websites[i]

		// Map hostname to website (if hostname is specified)
		if website.Hostname != "" {
			c.WeatherWebsites[website.Hostname] = website
			
			// Reload TLS configuration if certificates are provided
			if website.TLSCertPath != "" && website.TLSKeyPath != "" {
				cert, err := tls.LoadX509KeyPair(website.TLSCertPath, website.TLSKeyPath)
				if err != nil {
					c.logger.Errorf("Failed to load TLS certificate for website %s: %v", website.Name, err)
					// Continue without TLS for this website
				} else {
					// Build certificate chains if intermediates are present
					if cert.Leaf == nil && len(cert.Certificate) > 0 {
						// Parse the leaf certificate
						x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
						if err == nil {
							cert.Leaf = x509Cert
						}
					}
					
					c.tlsConfigs[website.Hostname] = &tls.Config{
						Certificates: []tls.Certificate{cert},
						MinVersion:   tls.VersionTLS12,
					}
					c.logger.Infof("Reloaded TLS certificate for website %s (hostname: %s)", website.Name, website.Hostname)
				}
			}
		}

		// Set default website (first one or one without hostname)
		if c.DefaultWebsite == nil || website.Hostname == "" {
			c.DefaultWebsite = website
		}

		// Validate snow device if snow is enabled and cache base distance
		if website.SnowEnabled {
			found := false
			for _, device := range c.Devices {
				if device.Name == website.SnowDeviceName {
					c.SnowBaseDistanceCache[website.ID] = float32(device.BaseSnowDistance)
					found = true
					break
				}
			}
			if !found {
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

	// DefaultWebsite can be nil if no websites are configured
	if c.DefaultWebsite == nil && len(websites) > 0 {
		return fmt.Errorf("no default website could be determined")
	}

	// Reload devices configuration as well
	cfgData, err := c.configProvider.LoadConfig()
	if err != nil {
		return fmt.Errorf("error reloading configuration: %v", err)
	}
	c.Devices = cfgData.Devices
	c.RefreshDeviceNames()
	
	// Re-check if any device has Aeris Weather enabled
	c.AerisWeatherEnabled = false
	for _, device := range c.Devices {
		if device.AerisEnabled {
			c.AerisWeatherEnabled = true
			break
		}
	}

	c.logger.Info("Website configuration reloaded successfully")
	return nil
}


// getCertificate returns the appropriate certificate based on SNI
func (c *Controller) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// Extract hostname from SNI
	hostname := hello.ServerName
	
	// Try to find exact match first
	if tlsConfig, exists := c.tlsConfigs[hostname]; exists && len(tlsConfig.Certificates) > 0 {
		return &tlsConfig.Certificates[0], nil
	}
	
	// If no exact match, try to find a wildcard certificate
	// This is a simple implementation - you might want to enhance it
	for configuredHost, tlsConfig := range c.tlsConfigs {
		if strings.HasPrefix(configuredHost, "*.") {
			domain := configuredHost[2:] // Remove "*."
			if strings.HasSuffix(hostname, domain) {
				return &tlsConfig.Certificates[0], nil
			}
		}
	}
	
	// If no SNI provided or no match found, try to return first available certificate
	if hostname == "" || len(c.tlsConfigs) > 0 {
		for _, tlsConfig := range c.tlsConfigs {
			if len(tlsConfig.Certificates) > 0 {
				return &tlsConfig.Certificates[0], nil
			}
		}
	}
	
	// If still no match, return error
	return nil, fmt.Errorf("no certificate found for hostname: %s", hostname)
}

// getDeviceNames returns a slice of all device names
func getDeviceNames(devices []config.DeviceData) []string {
	var names []string
	for _, device := range devices {
		names = append(names, device.Name)
	}
	return names
}

// Shared Database Query Methods

// Time constants for table selection
const (
	Day   = 24 * time.Hour
	Month = Day * 30
)

// fetchWeatherSpan queries the database for weather readings over a time span
// This is the shared logic used by both HTTP and gRPC handlers
func (c *Controller) fetchWeatherSpan(stationName string, span time.Duration, baseDistance float64) ([]types.BucketReading, error) {
	if !c.DBEnabled {
		return nil, fmt.Errorf("database not enabled")
	}

	// Reject queries for time spans greater than 1 year
	if span > 365*24*time.Hour {
		return nil, fmt.Errorf("time span exceeds maximum allowed duration of 1 year")
	}

	var dbFetchedReadings []types.BucketReading
	spanStart := time.Now().Add(-span)

	// Select appropriate table based on span duration
	// Target: Keep data points under ~300 for reasonable payload size and client rendering
	var tableName string
	switch {
	case span <= 6*time.Hour:
		// Up to 6 hours: 1-minute data (max 360 points)
		tableName = "weather_1m"
	case span > 6*time.Hour && span <= 48*time.Hour:
		// 6-48 hours: 5-minute data (max 576 points, typically ~200-300)
		tableName = "weather_5m"
	case span > 48*time.Hour && span <= 14*Day:
		// 2-14 days: hourly data (max 336 points)
		tableName = "weather_1h"
	default:
		// > 14 days: daily data (14-365 points)
		tableName = "weather_1d"
	}

	// Use raw SQL to avoid GORM query builder planning overhead (~45ms per query)
	// GORM's Table().Where() generates complex query plans that TimescaleDB re-plans every time
	query := fmt.Sprintf(`SELECT *, snowdistance FROM %s WHERE bucket > ? AND stationname = ? ORDER BY bucket`, tableName)
	err := c.DB.Raw(query, spanStart, stationName).Scan(&dbFetchedReadings).Error

	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	// Fetch smoothed snow depth estimates from snow_depth_est_5m table
	// These estimates use local quantile smoothing + rate limiting for physically plausible depth curves
	if len(dbFetchedReadings) > 0 && baseDistance > 0 {
		// Query smoothed estimates for the same time range
		var smoothedEstimates []struct {
			Time    time.Time `gorm:"column:time"`
			DepthIn float64   `gorm:"column:snow_depth_est_in"`
		}

		estimateQuery := `
			SELECT time, snow_depth_est_in
			FROM snow_depth_est_5m
			WHERE stationname = ? AND time >= ? AND time <= ?
			ORDER BY time
		`
		err := c.DB.Raw(estimateQuery, stationName, spanStart, time.Now()).Scan(&smoothedEstimates).Error
		if err == nil && len(smoothedEstimates) > 0 {
			// Create a map of timestamp to smoothed depth for fast lookup
			smoothedMap := make(map[int64]float32)
			for _, est := range smoothedEstimates {
				// Convert inches to mm to match existing data expectations
				depthMM := float32(est.DepthIn * 25.4)
				smoothedMap[est.Time.Unix()] = depthMM
			}

			// Populate snow depth from smoothed estimates
			for i := range dbFetchedReadings {
				timestamp := dbFetchedReadings[i].Bucket.Unix()
				if smoothedDepth, found := smoothedMap[timestamp]; found {
					dbFetchedReadings[i].SnowDepth = smoothedDepth
					// Keep raw snowdistance for reference if needed
				}
			}
		} else {
			// Fallback to raw depth calculation if no smoothed estimates available
			// (e.g., station not using SmoothedComputer or backfill not yet run)
			for i := range dbFetchedReadings {
				if dbFetchedReadings[i].SnowDistance > 0 {
					depth := float32(baseDistance) - dbFetchedReadings[i].SnowDistance
					dbFetchedReadings[i].SnowDepth = depth
				}
			}
		}
	}

	c.logger.Debugf("fetchWeatherSpan returned %d rows for station %s, span %v, table=%s",
		len(dbFetchedReadings), stationName, span, tableName)
	return dbFetchedReadings, nil
}

func (c *Controller) fetchLatestReading(stationName string, baseDistance float64) (*types.BucketReading, error) {
	if !c.DBEnabled {
		return nil, fmt.Errorf("database not enabled")
	}

	var dbFetchedReadings []types.BucketReading

	err := c.DB.Table("weather").
		Select("time AS bucket, *, (? - snowdistance) AS snowdepth", baseDistance).
		Where("stationname = ?", stationName).
		Where("time >= NOW() - INTERVAL '10 minutes'").
		Order("time desc").
		Limit(1).
		Find(&dbFetchedReadings).Error

	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	if len(dbFetchedReadings) == 0 {
		// Return sentinel error to distinguish from actual database errors
		return nil, fmt.Errorf("%w for station %s", ErrNoReadingsFound, stationName)
	}

	return &dbFetchedReadings[0], nil
}

// getSnowBaseDistanceForStation returns the snow base distance for a specific station
func (c *Controller) getSnowBaseDistanceForStation(stationName string) float64 {
	if c.DeviceManager != nil {
		return float64(c.DeviceManager.GetSnowBaseDistance(stationName))
	}
	// Fallback to default if no device manager
	return 0
}

// gRPC Weather Service Implementation

// GetWeatherSpan handles gRPC requests for weather data over a time span
func (c *Controller) GetWeatherSpan(ctx context.Context, request *weather.WeatherSpanRequest) (*weather.WeatherSpan, error) {
	// Validate station name
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := c.getSnowBaseDistanceForStation(request.StationName)

	// Use shared database fetching logic
	span := request.SpanDuration.AsDuration()
	dbFetchedReadings, err := c.fetchWeatherSpan(request.StationName, span, baseDistance)
	if err != nil {
		// Return InvalidArgument status for span duration errors
		if err.Error() == "time span exceeds maximum allowed duration of 1 year" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Transform readings to protobuf format
	readings := grpcutil.TransformBucketReadings(&dbFetchedReadings)

	spanResponse := &weather.WeatherSpan{
		SpanStart: timestamppb.New(time.Now().Add(-span)),
		Reading:   readings,
	}

	return spanResponse, nil
}

// GetLatestReading handles gRPC requests for the latest weather reading
func (c *Controller) GetLatestReading(ctx context.Context, request *weather.LatestReadingRequest) (*weather.WeatherReading, error) {
	// Validate station name
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := c.getSnowBaseDistanceForStation(request.StationName)

	// Use shared database fetching logic
	latestReading, err := c.fetchLatestReading(request.StationName, baseDistance)
	if err != nil {
		return nil, err
	}

	// Transform reading to protobuf format
	readings := grpcutil.TransformBucketReadings(&[]types.BucketReading{*latestReading})
	if len(readings) > 0 {
		return readings[0], nil
	}

	return nil, fmt.Errorf("no weather readings found")
}

// GetLiveWeather streams live weather data by polling the database every 3 seconds
func (c *Controller) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.WeatherV1_GetLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	if !c.DBEnabled {
		return fmt.Errorf("database not configured")
	}

	// Validate station name
	if err := grpcutil.ValidateStationRequest(req.StationName, c.DeviceManager); err != nil {
		return err
	}

	log.Infof("Starting live weather stream for client [%v] requesting station [%s]", p.Addr, req.StationName)

	// Poll database every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastReadingTime time.Time

	// Send initial reading immediately
	baseDistance := c.getSnowBaseDistanceForStation(req.StationName)
	if reading, err := c.fetchLatestReading(req.StationName, baseDistance); err == nil {
		grpcReadings := grpcutil.TransformBucketReadings(&[]types.BucketReading{*reading})
		if len(grpcReadings) > 0 {
			if err := stream.Send(grpcReadings[0]); err != nil {
				return err
			}
			lastReadingTime = reading.Bucket
			log.Debugf("Sent initial reading to client [%v] for station [%s]", p.Addr, req.StationName)
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Infof("Client [%v] disconnected from live weather stream for station [%s]", p.Addr, req.StationName)
			return nil
		case <-ticker.C:
			// Query latest reading from database
			reading, err := c.fetchLatestReading(req.StationName, baseDistance)
			if err != nil {
				// Log but don't disconnect client - station might be temporarily offline
				log.Debugf("No reading found for station %s: %v", req.StationName, err)
				continue
			}

			// Only send if it's a new reading (different timestamp)
			if reading.Bucket.After(lastReadingTime) {
				grpcReadings := grpcutil.TransformBucketReadings(&[]types.BucketReading{*reading})
				if len(grpcReadings) > 0 {
					if err := stream.Send(grpcReadings[0]); err != nil {
						log.Debugf("Error sending to client [%v]: %v", p.Addr, err)
						return err
					}
					lastReadingTime = reading.Bucket
					log.Debugf("Sent updated reading to client [%v] for station [%s]", p.Addr, req.StationName)
				}
			}
		}
	}
}
