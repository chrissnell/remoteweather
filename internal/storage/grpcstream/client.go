package grpcstream

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	weather "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ClientStorage implements the client-side gRPC storage for remote weather stations
type ClientStorage struct {
	endpoint      string
	tlsEnabled    bool
	stationID     string  // Cached station ID from server
	deviceName    string  // Local device name for config lookups
	configProvider config.ConfigProvider
	logger        *zap.SugaredLogger
	
	conn          *grpc.ClientConn
	client        weather.WeatherV1Client
	stream        weather.WeatherV1_SendWeatherReadingsClient
	
	mu            sync.RWMutex
	reconnecting  bool
	lastReconnect time.Time
}

// NewClient creates a new gRPC client storage instance
// When deviceName is empty, the client streams from all local weather stations
func NewClient(endpoint string, tlsEnabled bool, deviceName string, configProvider config.ConfigProvider, logger *zap.SugaredLogger) *ClientStorage {
	return &ClientStorage{
		endpoint:       endpoint,
		tlsEnabled:     tlsEnabled,
		deviceName:     deviceName,
		configProvider: configProvider,
		logger:         logger,
	}
}

// StartStorageEngine implements the StorageEngineInterface
func (c *ClientStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	c.logger.Info("starting gRPC client storage engine...")
	
	// Initialize connection and registration
	if err := c.initialize(ctx); err != nil {
		c.logger.Errorf("failed to initialize gRPC client: %v", err)
		// Continue anyway, will retry on reconnect
	}
	
	readingChan := make(chan types.Reading, 100)
	
	// Start processing readings
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.processReadings(ctx, readingChan)
	}()
	
	// Start health monitoring and reconnection
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.monitorConnection(ctx)
	}()
	
	return readingChan
}

// initialize establishes connection and registers with the server
func (c *ClientStorage) initialize(ctx context.Context) error {
	// Load or generate station ID
	if err := c.loadStationID(); err != nil {
		return fmt.Errorf("failed to load station ID: %w", err)
	}
	
	// Establish connection
	if err := c.connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	// Register with server
	if err := c.register(ctx); err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}
	
	// Start streaming
	if err := c.startStream(ctx); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	
	return nil
}

// loadStationID loads the station ID from persisted storage
func (c *ClientStorage) loadStationID() error {
	// Get SQLite provider to access the database
	cachedProvider, ok := c.configProvider.(*config.CachedConfigProvider)
	if !ok {
		c.logger.Debug("config provider is not cached, cannot load station ID")
		return nil
	}
	
	sqliteProvider, ok := cachedProvider.GetUnderlying().(*config.SQLiteProvider)
	if !ok {
		c.logger.Debug("underlying provider is not SQLite, cannot load station ID")
		return nil
	}
	
	db := sqliteProvider.GetDB()
	
	// Query for existing station ID
	var stationID string
	err := db.QueryRow(
		"SELECT station_id FROM storage_configs WHERE backend_type = 'grpcstream' LIMIT 1",
	).Scan(&stationID)
	
	if err == nil && stationID != "" {
		c.stationID = stationID
		c.logger.Infof("loaded existing station ID: %s", c.stationID)
	} else {
		c.logger.Debug("no existing station ID found, will obtain from server")
	}
	
	return nil
}

// persistStationID saves the station ID to the database
func (c *ClientStorage) persistStationID() error {
	// Get SQLite provider to access the database
	cachedProvider, ok := c.configProvider.(*config.CachedConfigProvider)
	if !ok {
		return fmt.Errorf("config provider is not cached")
	}
	
	sqliteProvider, ok := cachedProvider.GetUnderlying().(*config.SQLiteProvider)
	if !ok {
		return fmt.Errorf("underlying provider is not SQLite")
	}
	
	db := sqliteProvider.GetDB()
	
	// First check if a row exists
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM storage_configs WHERE backend_type = 'grpcstream'",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing config: %w", err)
	}
	
	if count > 0 {
		// Update existing row
		_, err = db.Exec(
			"UPDATE storage_configs SET station_id = ?, updated_at = CURRENT_TIMESTAMP WHERE backend_type = 'grpcstream'",
			c.stationID,
		)
	} else {
		// Insert new row
		_, err = db.Exec(
			"INSERT INTO storage_configs (config_id, backend_type, endpoint, tls_enabled, station_id, created_at, updated_at) VALUES ((SELECT id FROM configs WHERE name = 'default'), 'grpcstream', ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
			c.endpoint, c.tlsEnabled, c.stationID,
		)
	}
	
	if err != nil {
		return fmt.Errorf("failed to persist station ID: %w", err)
	}
	
	c.logger.Debugf("persisted station ID %s to database", c.stationID)
	return nil
}

// getGRPCReceiverDevices returns a list of grpcreceiver device names
// These should be filtered out to avoid re-streaming received data
func (c *ClientStorage) getGRPCReceiverDevices() ([]string, error) {
	cfgData, err := c.configProvider.LoadConfig()
	if err != nil {
		return nil, err
	}

	var grpcReceivers []string
	for _, device := range cfgData.Devices {
		if device.Type == "grpcreceiver" {
			grpcReceivers = append(grpcReceivers, device.Name)
		}
	}
	return grpcReceivers, nil
}

// connect establishes the gRPC connection
func (c *ClientStorage) connect(_ context.Context) error {
	var opts []grpc.DialOption
	
	// Configure TLS
	if c.tlsEnabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	
	// Add keepalive parameters
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}))
	
	c.logger.Infof("connecting to gRPC server at %s (TLS: %v)", c.endpoint, c.tlsEnabled)
	
	conn, err := grpc.NewClient(c.endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}
	
	c.conn = conn
	c.client = weather.NewWeatherV1Client(conn)
	
	return nil
}

// register registers the station with the server
func (c *ClientStorage) register(ctx context.Context) error {
	// Build registration config from device configuration
	regConfig := c.buildRegistrationConfig()
	
	c.logger.Infof("registering station with server (existing ID: %s)", c.stationID)
	
	resp, err := c.client.RegisterRemoteStation(ctx, regConfig)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}
	
	// Save station ID if new or changed
	if c.stationID != resp.StationId {
		oldID := c.stationID
		c.stationID = resp.StationId
		
		// Persist station ID to database
		if err := c.persistStationID(); err != nil {
			c.logger.Errorf("failed to persist station ID: %v", err)
			// Continue anyway, ID is cached in memory
		}
		
		if oldID == "" {
			c.logger.Infof("received new station ID from server: %s", c.stationID)
		} else {
			c.logger.Infof("station ID changed from %s to %s", oldID, c.stationID)
		}
	}
	
	c.logger.Infof("successfully registered with server as station %s", c.stationID)
	return nil
}

// buildRegistrationConfig builds the registration config from device settings
func (c *ClientStorage) buildRegistrationConfig() *weather.RemoteStationConfig {
	// If no device name specified, we're in multi-station forwarding mode
	if c.deviceName == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown-station"
		}

		return &weather.RemoteStationConfig{
			StationId:   c.stationID,
			StationName: hostname,
			StationType: "multi-station-forwarder",
		}
	}

	// Get the device configuration
	device, err := c.configProvider.GetDevice(c.deviceName)
	if err != nil {
		c.logger.Errorf("failed to get device config for %s: %v", c.deviceName, err)
		// Return minimal config
		return &weather.RemoteStationConfig{
			StationId:   c.stationID,
			StationName: c.deviceName,
			StationType: "remote",
		}
	}

	cfg := &weather.RemoteStationConfig{
		StationId:   c.stationID, // Empty for new registration
		StationName: device.Name,
		StationType: device.Type,
	}

	// APRS configuration from device
	if device.APRSEnabled {
		cfg.AprsEnabled = true
		cfg.AprsCallsign = device.APRSCallsign
		cfg.AprsPassword = device.APRSPasscode
	}

	// Weather Underground configuration from device
	if device.WUEnabled {
		cfg.WuEnabled = true
		cfg.WuStationId = device.WUStationID
		cfg.WuApiKey = device.WUPassword
	}

	// Aeris configuration from device
	if device.AerisEnabled {
		cfg.AerisEnabled = true
		cfg.AerisClientId = device.AerisAPIClientID
		cfg.AerisClientSecret = device.AerisAPIClientSecret
	}

	// PWS Weather configuration from device
	if device.PWSEnabled {
		cfg.PwsEnabled = true
		cfg.PwsStationId = device.PWSStationID
		cfg.PwsPassword = device.PWSPassword
	}

	return cfg
}

// startStream starts the weather reading stream
func (c *ClientStorage) startStream(ctx context.Context) error {
	stream, err := c.client.SendWeatherReadings(ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	
	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()
	
	c.logger.Info("weather reading stream established")
	return nil
}

// processReadings processes incoming readings and sends them to the server
func (c *ClientStorage) processReadings(ctx context.Context, readingChan <-chan types.Reading) {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("shutting down gRPC client reading processor")
			return
			
		case reading := <-readingChan:
			if err := c.sendReading(reading); err != nil {
				c.logger.Errorf("failed to send reading: %v", err)
				// Trigger reconnection if needed
				go c.reconnect(ctx)
			}
		}
	}
}

// sendReading sends a single reading to the server
func (c *ClientStorage) sendReading(reading types.Reading) error {
	// Filter out readings from grpcreceiver devices to avoid loops
	grpcReceivers, err := c.getGRPCReceiverDevices()
	if err != nil {
		c.logger.Warnf("Failed to get grpcreceiver list: %v", err)
		// Continue anyway - better to send data than drop it
	}

	for _, receiver := range grpcReceivers {
		if reading.StationName == receiver {
			c.logger.Debugf("Skipping reading from grpcreceiver device: %s", receiver)
			return nil // Not an error, just filtered
		}
	}

	c.mu.RLock()
	stream := c.stream
	c.mu.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream not initialized")
	}

	// Convert to protobuf
	pbReading := ConvertToProto(reading)

	// Add station ID
	pbReading.StationId = c.stationID

	// Send reading
	if err := stream.Send(pbReading); err != nil {
		return fmt.Errorf("stream send failed: %w", err)
	}

	c.logger.Debugf("sent reading from %s to gRPC server (station %s)", reading.StationName, c.stationID)
	return nil
}

// monitorConnection monitors the connection health and triggers reconnection
func (c *ClientStorage) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("shutting down gRPC client connection monitor")
			return
			
		case <-ticker.C:
			if c.conn != nil {
				state := c.conn.GetState()
				if state == connectivity.TransientFailure || state == connectivity.Shutdown {
					c.logger.Warnf("gRPC connection in state %v, triggering reconnection", state)
					go c.reconnect(ctx)
				}
			}
		}
	}
}

// reconnect handles reconnection logic
func (c *ClientStorage) reconnect(ctx context.Context) {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return // Already reconnecting
	}
	
	// Check if we recently tried to reconnect
	if time.Since(c.lastReconnect) < 10*time.Second {
		c.mu.Unlock()
		return
	}
	
	c.reconnecting = true
	c.lastReconnect = time.Now()
	c.mu.Unlock()
	
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()
	
	c.logger.Info("attempting to reconnect to gRPC server...")
	
	// Close existing connection
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	
	// Clear stream
	c.mu.Lock()
	c.stream = nil
	c.mu.Unlock()
	
	// Retry with exponential backoff
	backoff := time.Second
	maxBackoff := time.Minute
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		if err := c.initialize(ctx); err != nil {
			c.logger.Errorf("reconnection failed: %v, retrying in %v", err, backoff)
			
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			
			// Exponential backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			c.logger.Info("successfully reconnected to gRPC server")
			return
		}
	}
}

// CheckHealth checks the health of the gRPC client storage
func (c *ClientStorage) CheckHealth() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.conn == nil {
		return fmt.Errorf("no connection established")
	}
	
	state := c.conn.GetState()
	if state != connectivity.Ready && state != connectivity.Idle {
		return fmt.Errorf("connection not ready: %v", state)
	}
	
	if c.stream == nil {
		return fmt.Errorf("stream not established")
	}
	
	if c.stationID == "" {
		return fmt.Errorf("station not registered")
	}
	
	return nil
}