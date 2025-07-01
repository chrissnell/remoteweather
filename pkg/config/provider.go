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

	// APRS management
	GetStationAPRSConfigs() ([]StationAPRSData, error)
	GetStationAPRSConfig(deviceName string) (*StationAPRSData, error)
	AddStationAPRSConfig(config *StationAPRSData) error
	UpdateStationAPRSConfig(deviceName string, config *StationAPRSData) error
	DeleteStationAPRSConfig(deviceName string) error

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
	Name              string    `json:"name"`
	Type              string    `json:"type,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	Port              string    `json:"port,omitempty"`
	SerialDevice      string    `json:"serial_device,omitempty"`
	Baud              int       `json:"baud,omitempty"`
	WindDirCorrection int16     `json:"wind_dir_correction,omitempty"`
	BaseSnowDistance  int16     `json:"base_snow_distance,omitempty"`
	WebsiteID         *int      `json:"website_id,omitempty"`
	Solar             SolarData `json:"solar,omitempty"`
}

// SolarData holds configuration specific to solar calculations
type SolarData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

// StorageData holds the configuration for various storage backends
type StorageData struct {
	TimescaleDB *TimescaleDBData `json:"timescaledb,omitempty"`
	GRPC        *GRPCData        `json:"grpc,omitempty"`
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
}

// Storage backend configuration structs
type TimescaleDBData struct {
	ConnectionString string `json:"connection_string"`
}

type GRPCData struct {
	Cert           string `json:"cert,omitempty"`
	Key            string `json:"key,omitempty"`
	ListenAddr     string `json:"listen_addr,omitempty"`
	Port           int    `json:"port,omitempty"`
	PullFromDevice string `json:"pull_from_device,omitempty"`
}

// Per-station APRS configuration
// APRS server is global (configured as 'aprs' controller), only callsign and location are per-station
type StationAPRSData struct {
	DeviceName string    `json:"device_name,omitempty"`
	Enabled    bool      `json:"enabled,omitempty"`
	Callsign   string    `json:"callsign,omitempty"`
	Location   PointData `json:"location,omitempty"`
}

type PointData struct {
	Lat float64 `json:"latitude,omitempty"`
	Lon float64 `json:"longitude,omitempty"`
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
	APIClientID     string `json:"api_client_id"`
	APIClientSecret string `json:"api_client_secret"`
	APIEndpoint     string `json:"api_endpoint,omitempty"`
	Location        string `json:"location"`
}

type APRSData struct {
	Server string `json:"server,omitempty"`
}

// RESTServerData holds configuration for the REST server
// The REST server serves both REST API endpoints and weather websites
// It uses a single listener that routes based on Host header/SNI
type RESTServerData struct {
	HTTPPort          int    `json:"http_port,omitempty"`           // Single HTTP port for all websites
	HTTPSPort         *int   `json:"https_port,omitempty"`          // Optional HTTPS port for all websites
	DefaultListenAddr string `json:"default_listen_addr,omitempty"` // Listen address (default: 0.0.0.0)
	TLSCertPath       string `json:"tls_cert_path,omitempty"`       // Default TLS cert path
	TLSKeyPath        string `json:"tls_key_path,omitempty"`        // Default TLS key path
}

// WeatherWebsiteData represents a weather website configuration
// Websites are served by the single REST server and routed by hostname
type WeatherWebsiteData struct {
	ID               int    `json:"id,omitempty"`
	Name             string `json:"name"`
	Hostname         string `json:"hostname,omitempty"` // Domain name for this website (e.g., weather.example.com)
	PageTitle        string `json:"page_title,omitempty"`
	AboutStationHTML string `json:"about_station_html,omitempty"`
	SnowEnabled      bool   `json:"snow_enabled,omitempty"`
	SnowDeviceName   string `json:"snow_device_name,omitempty"`
	TLSCertPath      string `json:"tls_cert_path,omitempty"` // Optional per-site TLS cert (overrides server default)
	TLSKeyPath       string `json:"tls_key_path,omitempty"`  // Optional per-site TLS key (overrides server default)
}

type ManagementAPIData struct {
	Cert       string `json:"cert,omitempty"`
	Key        string `json:"key,omitempty"`
	Port       int    `json:"port,omitempty"`
	ListenAddr string `json:"listen_addr,omitempty"`
	AuthToken  string `json:"auth_token,omitempty"`
	EnableCORS bool   `json:"enable_cors,omitempty"`
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
		validTypes := []string{"campbellscientific", "davis", "snowgauge"}
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

		if !hasSerial && !hasNetwork {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("devices[%d]", i),
				Value:   device.Name,
				Message: "device must have either serial_device or both hostname and port configured",
			})
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

		// Validate solar configuration if present
		if device.Solar.Latitude != 0 || device.Solar.Longitude != 0 {
			if device.Solar.Latitude < -90 || device.Solar.Latitude > 90 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].solar.latitude", i),
					Value:   fmt.Sprintf("%.6f", device.Solar.Latitude),
					Message: "latitude must be between -90 and 90 degrees",
				})
			}
			if device.Solar.Longitude < -180 || device.Solar.Longitude > 180 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("devices[%d].solar.longitude", i),
					Value:   fmt.Sprintf("%.6f", device.Solar.Longitude),
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
			if controller.PWSWeather != nil {
				if controller.PWSWeather.StationID == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].pwsweather.station_id", i),
						Value:   "",
						Message: "PWS Weather station_id is required",
					})
				}
				if controller.PWSWeather.APIKey == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].pwsweather.api_key", i),
						Value:   "",
						Message: "PWS Weather api_key is required",
					})
				}
				if controller.PWSWeather.PullFromDevice != "" && !deviceNames[controller.PWSWeather.PullFromDevice] {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].pwsweather.pull_from_device", i),
						Value:   controller.PWSWeather.PullFromDevice,
						Message: "pull_from_device references non-existent device",
					})
				}
			}
		case "weatherunderground":
			if controller.WeatherUnderground != nil {
				if controller.WeatherUnderground.StationID == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].weatherunderground.station_id", i),
						Value:   "",
						Message: "Weather Underground station_id is required",
					})
				}
				if controller.WeatherUnderground.APIKey == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].weatherunderground.api_key", i),
						Value:   "",
						Message: "Weather Underground api_key is required",
					})
				}
				if controller.WeatherUnderground.PullFromDevice != "" && !deviceNames[controller.WeatherUnderground.PullFromDevice] {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].weatherunderground.pull_from_device", i),
						Value:   controller.WeatherUnderground.PullFromDevice,
						Message: "pull_from_device references non-existent device",
					})
				}
			}
		case "aerisweather":
			if controller.AerisWeather != nil {
				if controller.AerisWeather.APIClientID == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].aerisweather.api_client_id", i),
						Value:   "",
						Message: "Aeris Weather api_client_id is required",
					})
				}
				if controller.AerisWeather.APIClientSecret == "" {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("controllers[%d].aerisweather.api_client_secret", i),
						Value:   "",
						Message: "Aeris Weather api_client_secret is required",
					})
				}
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

// APRS management delegation methods

func (c *CachedConfigProvider) GetStationAPRSConfigs() ([]StationAPRSData, error) {
	return c.provider.GetStationAPRSConfigs()
}

func (c *CachedConfigProvider) GetStationAPRSConfig(deviceName string) (*StationAPRSData, error) {
	return c.provider.GetStationAPRSConfig(deviceName)
}

func (c *CachedConfigProvider) AddStationAPRSConfig(config *StationAPRSData) error {
	err := c.provider.AddStationAPRSConfig(config)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

func (c *CachedConfigProvider) UpdateStationAPRSConfig(deviceName string, config *StationAPRSData) error {
	err := c.provider.UpdateStationAPRSConfig(deviceName, config)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}

func (c *CachedConfigProvider) DeleteStationAPRSConfig(deviceName string) error {
	err := c.provider.DeleteStationAPRSConfig(deviceName)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}
