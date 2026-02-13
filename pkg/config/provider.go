// Package config provides configuration management with support for multiple data sources and caching.
package config

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ConfigProvider defines the interface for configuration data sources
type ConfigProvider interface {
	// Load complete configuration
	LoadConfig() (*ConfigData, error)

	// Get specific configuration sections
	GetDevices() ([]DeviceData, error)
	GetStorageConfig() (*StorageData, error)
	GetControllers() ([]ControllerData, error)

	// Individual device management
	AddDevice(device *DeviceData) error
	UpdateDevice(name string, device *DeviceData) error
	DeleteDevice(name string) error
	GetDevice(name string) (*DeviceData, error)

	// Individual storage management
	AddStorageConfig(storageType string, config interface{}) error
	UpdateStorageConfig(storageType string, config interface{}) error
	DeleteStorageConfig(storageType string) error

	// Individual controller management
	AddController(controller *ControllerData) error
	UpdateController(controllerType string, controller *ControllerData) error
	DeleteController(controllerType string) error
	GetController(controllerType string) (*ControllerData, error)

	// Weather website management
	GetWeatherWebsites() ([]WeatherWebsiteData, error)
	GetWeatherWebsite(id int) (*WeatherWebsiteData, error)
	AddWeatherWebsite(website *WeatherWebsiteData) error
	UpdateWeatherWebsite(id int, website *WeatherWebsiteData) error
	DeleteWeatherWebsite(id int) error

	// Sun times management (pre-calculated sunrise/sunset)
	GetSunTimes(stationName string, dayOfYear int) (sunrise, sunset int, err error)
	HasSunTimes(stationName string) (bool, error)
	PopulateSunTimes(stationName string, latitude, longitude float64, calculator func(dayOfYear int, lat, lon float64) (sunrise, sunset int, err error)) error
	DeleteSunTimes(stationName string) error

	// Configuration management (for future SQLite-specific operations)
	IsReadOnly() bool
	Close() error
}

// CachedConfigProvider wraps any ConfigProvider with caching
type CachedConfigProvider struct {
	provider    ConfigProvider
	cache       *ConfigData
	cacheMutex  sync.RWMutex
	lastLoaded  time.Time
	cacheExpiry time.Duration
}

// NewCachedProvider creates a new cached config provider wrapper
func NewCachedProvider(provider ConfigProvider, cacheExpiry time.Duration) *CachedConfigProvider {
	if cacheExpiry == 0 {
		cacheExpiry = 30 * time.Second // Default cache expiry
	}

	return &CachedConfigProvider{
		provider:    provider,
		cacheExpiry: cacheExpiry,
	}
}

// LoadConfig loads configuration with caching
func (c *CachedConfigProvider) LoadConfig() (*ConfigData, error) {
	c.cacheMutex.RLock()
	if c.cache != nil && time.Since(c.lastLoaded) < c.cacheExpiry {
		defer c.cacheMutex.RUnlock()
		return c.cache, nil
	}
	c.cacheMutex.RUnlock()

	// Cache miss or expired, reload
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	// Double-check in case another goroutine loaded while we waited
	if c.cache != nil && time.Since(c.lastLoaded) < c.cacheExpiry {
		return c.cache, nil
	}

	config, err := c.provider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate the loaded configuration
	if validationErrors := ValidateConfig(config); len(validationErrors) > 0 {
		var errorMessages []string
		for _, ve := range validationErrors {
			errorMessages = append(errorMessages, ve.Error())
		}
		return nil, fmt.Errorf("configuration validation failed:\n  - %s",
			strings.Join(errorMessages, "\n  - "))
	}

	c.cache = config
	c.lastLoaded = time.Now()
	return config, nil
}

// GetDevices returns cached device configurations
func (c *CachedConfigProvider) GetDevices() ([]DeviceData, error) {
	config, err := c.LoadConfig()
	if err != nil {
		return nil, err
	}
	return config.Devices, nil
}

// GetStorageConfig returns cached storage configuration
func (c *CachedConfigProvider) GetStorageConfig() (*StorageData, error) {
	config, err := c.LoadConfig()
	if err != nil {
		return nil, err
	}
	return &config.Storage, nil
}

// GetControllers returns cached controller configurations
func (c *CachedConfigProvider) GetControllers() ([]ControllerData, error) {
	config, err := c.LoadConfig()
	if err != nil {
		return nil, err
	}
	return config.Controllers, nil
}

// IsReadOnly delegates to the underlying provider
func (c *CachedConfigProvider) IsReadOnly() bool {
	return c.provider.IsReadOnly()
}

// Close delegates to the underlying provider and clears cache
func (c *CachedConfigProvider) Close() error {
	c.cacheMutex.Lock()
	c.cache = nil
	c.cacheMutex.Unlock()
	return c.provider.Close()
}

// GetUnderlying returns the underlying config provider
func (c *CachedConfigProvider) GetUnderlying() ConfigProvider {
	return c.provider
}

// InvalidateCache forces a reload on the next access
func (c *CachedConfigProvider) InvalidateCache() {
	c.cacheMutex.Lock()
	c.cache = nil
	c.cacheMutex.Unlock()
}

// ConfigData represents the complete configuration structure
// This mirrors the main Config struct but is in our config package
type ConfigData struct {
	Devices     []DeviceData     `json:"devices"`
	Storage     StorageData      `json:"storage,omitempty"`
	Controllers []ControllerData `json:"controllers,omitempty"`
}

// DeviceData holds configuration specific to data collection devices
type DeviceData struct {
	ID                int     `json:"id,omitempty"`
	Name              string  `json:"name"`
	Type              string  `json:"type,omitempty"`
	Enabled           bool    `json:"enabled"`
	Hostname          string  `json:"hostname,omitempty"`
	Port              string  `json:"port,omitempty"`
	SerialDevice      string  `json:"serial_device,omitempty"`
	Baud              int     `json:"baud,omitempty"`
	WindDirCorrection int16   `json:"wind_dir_correction,omitempty"`
	BaseSnowDistance  int16   `json:"base_snow_distance,omitempty"`
	WebsiteID         *int    `json:"website_id,omitempty"`
	Latitude          float64 `json:"latitude,omitempty"`
	Longitude         float64 `json:"longitude,omitempty"`
	Altitude          float64 `json:"altitude,omitempty"`
	APRSEnabled       bool    `json:"aprs_enabled,omitempty"`
	APRSCallsign      string  `json:"aprs_callsign,omitempty"`
	TLSCertPath       string  `json:"tls_cert_path,omitempty"`
	TLSKeyPath        string  `json:"tls_key_path,omitempty"`
	Path              string  `json:"path,omitempty"`

	// PWS Weather fields
	PWSEnabled        bool   `json:"pws_enabled,omitempty"`
	PWSStationID      string `json:"pws_station_id,omitempty"`
	PWSPassword       string `json:"pws_password,omitempty"`
	PWSUploadInterval int    `json:"pws_upload_interval,omitempty"`
	PWSAPIEndpoint    string `json:"pws_api_endpoint,omitempty"`

	// Weather Underground fields
	WUEnabled        bool   `json:"wu_enabled,omitempty"`
	WUStationID      string `json:"wu_station_id,omitempty"`
	WUPassword       string `json:"wu_password,omitempty"`
	WUUploadInterval int    `json:"wu_upload_interval,omitempty"`
	WUAPIEndpoint    string `json:"wu_api_endpoint,omitempty"`

	// APRS additional fields
	APRSPasscode    string `json:"aprs_passcode,omitempty"`
	APRSSymbolTable string `json:"aprs_symbol_table,omitempty"`
	APRSSymbolCode  string `json:"aprs_symbol_code,omitempty"`
	APRSComment     string `json:"aprs_comment,omitempty"`
	APRSServer      string `json:"aprs_server,omitempty"`

	// Aeris Weather fields
	AerisEnabled         bool   `json:"aeris_enabled,omitempty"`
	AerisAPIClientID     string `json:"aeris_api_client_id,omitempty"`
	AerisAPIClientSecret string `json:"aeris_api_client_secret,omitempty"`
	AerisAPIEndpoint     string `json:"aeris_api_endpoint,omitempty"`

	// WeatherLink Live fields
	WLLHost          string `json:"wll_host,omitempty"`
	WLLPort          int    `json:"wll_port,omitempty"`
	WLLBroadcast     bool   `json:"wll_broadcast,omitempty"`
	WLLSensorMapping string `json:"wll_sensor_mapping,omitempty"`
	WLLPollInterval  int    `json:"wll_poll_interval,omitempty"`
}

// StorageData holds the configuration for various storage backends
type StorageData struct {
	TimescaleDB *TimescaleDBData `json:"timescaledb,omitempty"`
	GRPC        *GRPCData        `json:"grpc,omitempty"`
	GRPCStream  *GRPCStreamData  `json:"grpcstream,omitempty"`
}

// ControllerData holds the configuration for various controller backends
type ControllerData struct {
	Type               string                  `json:"type,omitempty"`
	PWSWeather         *PWSWeatherData         `json:"pwsweather,omitempty"`
	WeatherUnderground *WeatherUndergroundData `json:"weatherunderground,omitempty"`
	AerisWeather       *AerisWeatherData       `json:"aerisweather,omitempty"`
	RESTServer         *RESTServerData         `json:"rest,omitempty"`
	ManagementAPI      *ManagementAPIData      `json:"management,omitempty"`
	APRS               *APRSData               `json:"aprs,omitempty"`
	SnowCache          *SnowCacheData          `json:"snowcache,omitempty"`
}

// StorageHealthData holds health status information for storage backends
type StorageHealthData struct {
	LastCheck time.Time `json:"last_check,omitempty"`
	Status    string    `json:"status,omitempty"` // "healthy", "unhealthy", "stale", "unknown"
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Storage backend configuration structs
type TimescaleDBData struct {
	// Individual DSN components
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
	Timezone string `json:"timezone,omitempty"`

	Health *StorageHealthData `json:"health,omitempty"`
}

// GetConnectionString forms a DSN from individual components
func (td *TimescaleDBData) GetConnectionString() string {
	// Form DSN from individual components
	var parts []string

	if td.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", td.Host))
	}
	if td.Port > 0 {
		parts = append(parts, fmt.Sprintf("port=%d", td.Port))
	}
	if td.Database != "" {
		parts = append(parts, fmt.Sprintf("dbname=%s", td.Database))
	}
	if td.User != "" {
		parts = append(parts, fmt.Sprintf("user=%s", td.User))
	}
	if td.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", td.Password))
	}
	if td.SSLMode != "" {
		parts = append(parts, fmt.Sprintf("sslmode=%s", td.SSLMode))
	}
	if td.Timezone != "" {
		parts = append(parts, fmt.Sprintf("TimeZone=%s", td.Timezone))
	}

	return strings.Join(parts, " ")
}

type GRPCData struct {
	Cert           string             `json:"cert,omitempty"`
	Key            string             `json:"key,omitempty"`
	ListenAddr     string             `json:"listen_addr,omitempty"`
	Port           int                `json:"port,omitempty"`
	PullFromDevice string             `json:"pull_from_device,omitempty"`
	Health         *StorageHealthData `json:"health,omitempty"`
}

type GRPCStreamData struct {
	Endpoint   string             `json:"endpoint"`
	TLSEnabled bool               `json:"tls_enabled"`
	Health     *StorageHealthData `json:"health,omitempty"`
}

// Controller configuration structs
type PWSWeatherData struct {
	StationID      string `json:"station_id,omitempty"`
	APIKey         string `json:"api_key,omitempty"`
	APIEndpoint    string `json:"api_endpoint,omitempty"`
	UploadInterval string `json:"upload_interval,omitempty"`
	PullFromDevice string `json:"pull_from_device,omitempty"`
}

type WeatherUndergroundData struct {
	StationID      string `json:"station_id,omitempty"`
	APIKey         string `json:"api_key,omitempty"`
	UploadInterval string `json:"upload_interval,omitempty"`
	PullFromDevice string `json:"pull_from_device,omitempty"`
	APIEndpoint    string `json:"api_endpoint,omitempty"`
}

type AerisWeatherData struct {
	APIClientID     string  `json:"api_client_id"`
	APIClientSecret string  `json:"api_client_secret"`
	APIEndpoint     string  `json:"api_endpoint,omitempty"`
	Latitude        float64 `json:"latitude,omitempty"`
	Longitude       float64 `json:"longitude,omitempty"`
}

type APRSData struct {
	Server string             `json:"server,omitempty"`
	Health *StorageHealthData `json:"health,omitempty"`
}

// SnowCacheData holds configuration for the snow cache controller
// The snow cache controller runs background calculations for snow accumulation
// using PELT change point detection for 72h and seasonal totals
type SnowCacheData struct {
	StationName     string  `json:"station_name,omitempty"`      // Name of the weather station with snow depth sensor
	BaseDistance    float64 `json:"base_distance,omitempty"`     // Base snow distance calibration value (mm)
	SmoothingWindow int     `json:"smoothing_window,omitempty"`  // Hours of data for median filtering (default: 5)
	Penalty         float64 `json:"penalty,omitempty"`           // PELT penalty parameter (default: 3.0)
	MinAccumulation float64 `json:"min_accumulation,omitempty"`  // Minimum accumulation threshold in mm (default: 5.0)
	MinSegmentSize  int     `json:"min_segment_size,omitempty"`  // Minimum segment size for PELT (default: 2)
}

// RESTServerData holds configuration for the REST server
// The REST server serves both REST API endpoints and weather websites
// It uses a single listener that routes based on Host header/SNI
type RESTServerData struct {
	// HTTP/HTTPS configuration
	HTTPPort          int    `json:"http_port,omitempty"`           // Single HTTP port for all websites
	HTTPSPort         *int   `json:"https_port,omitempty"`          // Optional HTTPS port for all websites
	DefaultListenAddr string `json:"default_listen_addr,omitempty"` // Listen address (default: 0.0.0.0)

	// gRPC configuration
	GRPCPort       int    `json:"grpc_port,omitempty"`        // gRPC listener port (default: 50051)
	GRPCListenAddr string `json:"grpc_listen_addr,omitempty"` // gRPC listen address (default: 0.0.0.0)
	GRPCCertPath   string `json:"grpc_cert_path,omitempty"`   // Optional gRPC TLS certificate path
	GRPCKeyPath    string `json:"grpc_key_path,omitempty"`    // Optional gRPC TLS key path
}

// WeatherWebsiteData represents a weather website configuration
// Websites are served by the single REST server and routed by hostname
type WeatherWebsiteData struct {
	ID                   int    `json:"id,omitempty"`
	Name                 string `json:"name"`
	DeviceID             *int   `json:"device_id,omitempty"`   // Device ID (foreign key to devices.id) that provides data for this website
	DeviceName           string `json:"device_name,omitempty"` // Device name (populated from join, not stored)
	Hostname             string `json:"hostname,omitempty"`    // Domain name for this website (e.g., weather.example.com)
	PageTitle            string `json:"page_title,omitempty"`
	AboutStationHTML     string `json:"about_station_html,omitempty"`
	SnowEnabled          bool   `json:"snow_enabled,omitempty"`
	SnowDeviceName       string `json:"snow_device_name,omitempty"`
	AirQualityEnabled    bool   `json:"air_quality_enabled,omitempty"`
	AirQualityDeviceName string `json:"air_quality_device_name,omitempty"`
	TLSCertPath          string `json:"tls_cert_path,omitempty"` // Optional per-site TLS cert (overrides server default)
	TLSKeyPath           string `json:"tls_key_path,omitempty"`  // Optional per-site TLS key (overrides server default)
	IsPortal             bool   `json:"is_portal,omitempty"`     // Whether this website is a weather management portal
	AppleAppID           string `json:"apple_app_id,omitempty"`  // Numeric App Store ID for iOS Smart Banner
}

type ManagementAPIData struct {
	Cert       string `json:"cert,omitempty"`
	Key        string `json:"key,omitempty"`
	Port       int    `json:"port,omitempty"`
	ListenAddr string `json:"listen_addr,omitempty"`
	AuthToken  string `json:"auth_token,omitempty"`
}

// ValidateConfig performs comprehensive validation of configuration data
func ValidateConfig(config *ConfigData) []ValidationError {
	var errors []ValidationError

	// Validate devices
	deviceNames := make(map[string]bool)
	for i, device := range config.Devices {
		// Check for duplicate device names
		if deviceNames[device.Name] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("devices[%d].name", i),
				Value:   device.Name,
				Message: "duplicate device name",
			})
		}
		deviceNames[device.Name] = true

		// Validate device name
		if device.Name == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("devices[%d].name", i),
				Value:   "",
				Message: "device name is required",
			})
		}

		// Validate device type
		validTypes := []string{"campbellscientific", "davis", "snowgauge", "ambient-customized", "grpcreceiver", "airgradient"}
		if !contains(validTypes, device.Type) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("devices[%d].type", i),
				Value:   device.Type,
				Message: fmt.Sprintf("invalid device type, must be one of: %v", validTypes),
			})
		}

		// Validate connection settings
		hasSerial := device.SerialDevice != ""
		hasNetwork := device.Hostname != "" && device.Port != ""
		hasAmbientCustomized := device.Type == "ambient-customized" && device.Port != ""
		hasGRPCReceiver := device.Type == "grpcreceiver" && device.Port != ""
		hasAirGradient := device.Type == "airgradient" && device.Hostname != ""

		if !hasSerial && !hasNetwork && !hasAmbientCustomized && !hasGRPCReceiver && !hasAirGradient {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("devices[%d]", i),
				Value:   device.Name,
				Message: "device must have either serial_device or both hostname and port configured",
			})
		}

		// Set default port for airgradient if not specified
		if device.Type == "airgradient" && device.Hostname != "" && device.Port == "" {
			device.Port = "80"
		}

		// Validate snow gauge specific settings
		if device.Type == "snowgauge" {
			if device.BaseSnowDistance <= 0 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].base_snow_distance", i),
					Value:   fmt.Sprintf("%d", device.BaseSnowDistance),
					Message: "snow gauge must have base_snow_distance > 0",
				})
			}
		}

		// Validate ambient-customized specific settings
		if device.Type == "ambient-customized" && device.Path != "" {
			// Ensure path starts with /
			if !strings.HasPrefix(device.Path, "/") {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].path", i),
					Value:   device.Path,
					Message: "path must start with /",
				})
			}
		}

		// Validate location configuration if present
		if device.Latitude != 0 || device.Longitude != 0 {
			if device.Latitude < -90 || device.Latitude > 90 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].latitude", i),
					Value:   fmt.Sprintf("%.6f", device.Latitude),
					Message: "latitude must be between -90 and 90 degrees",
				})
			}
			if device.Longitude < -180 || device.Longitude > 180 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].longitude", i),
					Value:   fmt.Sprintf("%.6f", device.Longitude),
					Message: "longitude must be between -180 and 180 degrees",
				})
			}
		}
	}

	// Validate controllers
	for i, controller := range config.Controllers {
		if controller.Type == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("controllers[%d].type", i),
				Value:   "",
				Message: "controller type is required",
			})
			continue
		}

		// Validate controller-specific configurations
		switch controller.Type {
		case "rest":
			if controller.RESTServer != nil {
				// REST server now serves multiple websites, each with their own ports
				// Basic validation for listen address format if provided
				if controller.RESTServer.DefaultListenAddr != "" {
					// Could add IP address validation here if needed
				}
				// Note: Website-specific validation will be handled separately
				// since websites are now independent entities with their own validation
			}
		case "management":
			if controller.ManagementAPI != nil {
				if controller.ManagementAPI.Port <= 0 || controller.ManagementAPI.Port > 65535 {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].management.port", i),
						Value:   fmt.Sprintf("%d", controller.ManagementAPI.Port),
						Message: "port must be between 1 and 65535",
					})
				}
			}
		case "pwsweather":
			// PWS Weather controller validation
			// Device-specific credentials are now validated at the device level
			if controller.PWSWeather != nil {
				// Only validate API endpoint if provided
				// Other validations moved to device level
			}
		case "weatherunderground":
			// Weather Underground controller validation
			// Device-specific credentials are now validated at the device level
			if controller.WeatherUnderground != nil {
				// Only validate API endpoint if provided
				// Other validations moved to device level
			}
		case "aerisweather":
			// Aeris Weather controller validation
			// Device-specific credentials are now validated at the device level
			if controller.AerisWeather != nil {
				// Only validate API endpoint if provided
				// Other validations moved to device level
			}
		}
	}

	return errors
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (value: %s)", ve.Field, ve.Message, ve.Value)
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Individual device management methods

// GetDevice retrieves a specific device by name
func (c *CachedConfigProvider) GetDevice(name string) (*DeviceData, error) {
	return c.provider.GetDevice(name)
}

// AddDevice adds a new device and invalidates cache
func (c *CachedConfigProvider) AddDevice(device *DeviceData) error {
	err := c.provider.AddDevice(device)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// UpdateDevice updates an existing device and invalidates cache
func (c *CachedConfigProvider) UpdateDevice(name string, device *DeviceData) error {
	err := c.provider.UpdateDevice(name, device)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// DeleteDevice removes a device and invalidates cache
func (c *CachedConfigProvider) DeleteDevice(name string) error {
	err := c.provider.DeleteDevice(name)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// Individual storage management methods

// AddStorageConfig adds a new storage configuration and invalidates cache
func (c *CachedConfigProvider) AddStorageConfig(storageType string, config interface{}) error {
	err := c.provider.AddStorageConfig(storageType, config)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// UpdateStorageConfig updates an existing storage configuration and invalidates cache
func (c *CachedConfigProvider) UpdateStorageConfig(storageType string, config interface{}) error {
	err := c.provider.UpdateStorageConfig(storageType, config)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// DeleteStorageConfig removes a storage configuration and invalidates cache
func (c *CachedConfigProvider) DeleteStorageConfig(storageType string) error {
	err := c.provider.DeleteStorageConfig(storageType)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// Individual controller management methods

// GetController retrieves a specific controller by type
func (c *CachedConfigProvider) GetController(controllerType string) (*ControllerData, error) {
	return c.provider.GetController(controllerType)
}

// AddController adds a new controller and invalidates cache
func (c *CachedConfigProvider) AddController(controller *ControllerData) error {
	err := c.provider.AddController(controller)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// UpdateController updates an existing controller and invalidates cache
func (c *CachedConfigProvider) UpdateController(controllerType string, controller *ControllerData) error {
	err := c.provider.UpdateController(controllerType, controller)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// DeleteController removes a controller and invalidates cache
func (c *CachedConfigProvider) DeleteController(controllerType string) error {
	err := c.provider.DeleteController(controllerType)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// Weather website management methods

// GetWeatherWebsites retrieves all weather websites
func (c *CachedConfigProvider) GetWeatherWebsites() ([]WeatherWebsiteData, error) {
	return c.provider.GetWeatherWebsites()
}

// GetWeatherWebsite retrieves a specific weather website by ID
func (c *CachedConfigProvider) GetWeatherWebsite(id int) (*WeatherWebsiteData, error) {
	return c.provider.GetWeatherWebsite(id)
}

// AddWeatherWebsite adds a new weather website and invalidates cache
func (c *CachedConfigProvider) AddWeatherWebsite(website *WeatherWebsiteData) error {
	err := c.provider.AddWeatherWebsite(website)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// UpdateWeatherWebsite updates an existing weather website and invalidates cache
func (c *CachedConfigProvider) UpdateWeatherWebsite(id int, website *WeatherWebsiteData) error {
	err := c.provider.UpdateWeatherWebsite(id, website)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// DeleteWeatherWebsite removes a weather website and invalidates cache
func (c *CachedConfigProvider) DeleteWeatherWebsite(id int) error {
	err := c.provider.DeleteWeatherWebsite(id)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

// Sun times management methods (delegate to underlying provider, no caching needed)

// GetSunTimes retrieves sunrise/sunset times for a station on a given day
func (c *CachedConfigProvider) GetSunTimes(stationName string, dayOfYear int) (sunrise, sunset int, err error) {
	return c.provider.GetSunTimes(stationName, dayOfYear)
}

// HasSunTimes checks if sun times have been populated for a station
func (c *CachedConfigProvider) HasSunTimes(stationName string) (bool, error) {
	return c.provider.HasSunTimes(stationName)
}

// PopulateSunTimes calculates and stores sunrise/sunset times for a station
func (c *CachedConfigProvider) PopulateSunTimes(stationName string, latitude, longitude float64, calculator func(dayOfYear int, lat, lon float64) (sunrise, sunset int, err error)) error {
	return c.provider.PopulateSunTimes(stationName, latitude, longitude, calculator)
}

// DeleteSunTimes removes all sun times for a station
func (c *CachedConfigProvider) DeleteSunTimes(stationName string) error {
	return c.provider.DeleteSunTimes(stationName)
}
