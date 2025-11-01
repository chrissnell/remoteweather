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
	"github.com/gorilla/mux"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	// Register the weather service and reflection
	weather.RegisterWeatherV1Server(ctrl.GRPCServer, ctrl)
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

// StartController starts the unified REST/gRPC server using cmux
func (c *Controller) StartController() error {
	log.Info("Starting unified REST/gRPC server...")
	
	// Determine if we should use HTTPS based on TLS configurations
	useHTTPS := len(c.tlsConfigs) > 0
	
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		// Create the main listener
		listenAddr := fmt.Sprintf("%s:%d", c.restConfig.DefaultListenAddr, c.restConfig.HTTPPort)
		l, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Errorf("Failed to create listener: %v", err)
			return
		}
		defer l.Close()

		if useHTTPS {
			// HTTPS + gRPC mode
			log.Infof("Starting unified server on %s (HTTPS and gRPC) with %d configured certificates", listenAddr, len(c.tlsConfigs))
			
			// Create TLS config with SNI support
			tlsConfig := &tls.Config{
				GetCertificate: c.getCertificate,
				// Don't advertise h2 in ALPN when using cmux
				// cmux handles the protocol detection internally
				NextProtos: []string{"http/1.1"},
			}
			
			// Wrap listener with TLS
			tlsListener := tls.NewListener(l, tlsConfig)
			
			// Create cmux multiplexer on TLS listener
			m := cmux.New(tlsListener)
			
			// Match connections:
			// - gRPC (HTTP/2 with "application/grpc" content-type)
			// - HTTPS (HTTP/1.1 and HTTP/2 without gRPC content-type)
			grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
			httpsL := m.Match(cmux.Any())
			
			// Configure and start HTTPS server
			c.Server.Handler = c.setupRouter()
			go func() {
				if err := c.Server.Serve(httpsL); err != http.ErrServerClosed {
					log.Errorf("HTTPS server error: %v", err)
				}
			}()
			
			// Start gRPC server
			go func() {
				if err := c.GRPCServer.Serve(grpcL); err != nil {
					log.Errorf("gRPC server error: %v", err)
				}
			}()
			
			// Start serving connections
			if err := m.Serve(); err != nil && !strings.Contains(err.Error(), "closed") {
				log.Errorf("cmux serve error: %v", err)
			}
		} else {
			// HTTP + gRPC mode (original behavior when no TLS)
			log.Infof("Starting unified server on %s (HTTP and gRPC)", listenAddr)
			
			// Create cmux multiplexer
			m := cmux.New(l)
			
			// Match connections:
			// - gRPC (HTTP/2 with "application/grpc" content-type)
			// - HTTP/1.1 and HTTP/2 without gRPC content-type
			grpcL := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
			httpL := m.Match(cmux.Any())
			
			// Start HTTP server
			go func() {
				if err := c.Server.Serve(httpL); err != http.ErrServerClosed {
					log.Errorf("HTTP server error: %v", err)
				}
			}()
			
			// Start gRPC server
			go func() {
				if err := c.GRPCServer.Serve(grpcL); err != nil {
					log.Errorf("gRPC server error: %v", err)
				}
			}()
			
			// Start serving connections
			if err := m.Serve(); err != nil && !strings.Contains(err.Error(), "closed") {
				log.Errorf("cmux serve error: %v", err)
			}
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-c.ctx.Done()
		log.Info("Shutting down servers...")
		
		// Shutdown HTTP/HTTPS server
		c.Server.Shutdown(context.Background())
		
		// Stop gRPC server
		c.GRPCServer.GracefulStop()
	}()

	return nil
}

// setupRouter configures the HTTP router with all endpoints
func (c *Controller) setupRouter() *mux.Router {
	router := mux.NewRouter()

	// Add middleware to identify the website based on Host header
	router.Use(c.websiteMiddleware)

	// Add HTTP logging middleware
	router.Use(c.httpLoggingMiddleware)

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

	// Station API endpoints
	router.HandleFunc("/api/stations", c.handlers.GetStations)
	router.HandleFunc("/api/remote-stations", c.handlers.GetRemoteStations)

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

	var dbFetchedReadings []types.BucketReading
	spanStart := time.Now().Add(-span)

	// Select appropriate table based on span duration
	var tableName string
	switch {
	case span < 1*Day:
		tableName = "weather_1m"
	case span >= 1*Day && span < 7*Day:
		tableName = "weather_5m"
	case span >= 7*Day && span < 2*Month:
		tableName = "weather_1h"
	default:
		tableName = "weather_1h"
	}

	err := c.DB.Table(tableName).
		Select("*, (? - snowdistance) AS snowdepth", baseDistance).
		Where("bucket > ?", spanStart).
		Where("stationname = ?", stationName).
		Order("bucket").
		Find(&dbFetchedReadings).Error

	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	c.logger.Debugf("fetchWeatherSpan returned %d rows for station %s, span %v", len(dbFetchedReadings), stationName, span)
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
		return nil, fmt.Errorf("no weather readings found for station %s", stationName)
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
		return nil, err
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

// GetLiveWeather is not implemented for this controller since it's database-based
func (c *Controller) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.WeatherV1_GetLiveWeatherServer) error {
	return fmt.Errorf("live weather streaming not supported by database-based controller")
}
