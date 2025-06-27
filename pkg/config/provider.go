package config

// ConfigProvider defines the interface for configuration data sources
type ConfigProvider interface {
	// Load complete configuration
	LoadConfig() (*ConfigData, error)

	// Get specific configuration sections
	GetDevices() ([]DeviceData, error)
	GetStorageConfig() (*StorageData, error)
	GetControllers() ([]ControllerData, error)

	// Configuration management (for future SQLite-specific operations)
	IsReadOnly() bool
	Close() error
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
	InfluxDB    *InfluxDBData    `json:"influxdb,omitempty"`
	TimescaleDB *TimescaleDBData `json:"timescaledb,omitempty"`
	GRPC        *GRPCData        `json:"grpc,omitempty"`
	APRS        *APRSData        `json:"aprs,omitempty"`
}

// ControllerData holds the configuration for various controller backends
type ControllerData struct {
	Type               string                  `json:"type,omitempty"`
	PWSWeather         *PWSWeatherData         `json:"pwsweather,omitempty"`
	WeatherUnderground *WeatherUndergroundData `json:"weatherunderground,omitempty"`
	AerisWeather       *AerisWeatherData       `json:"aerisweather,omitempty"`
	RESTServer         *RESTServerData         `json:"rest,omitempty"`
	ManagementAPI      *ManagementAPIData      `json:"management,omitempty"`
}

// Storage backend configuration structs
type InfluxDBData struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database"`
	Port     int    `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

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

type APRSData struct {
	Callsign     string    `json:"callsign,omitempty"`
	Passcode     string    `json:"passcode,omitempty"`
	APRSISServer string    `json:"aprs_is_server,omitempty"`
	Location     PointData `json:"location,omitempty"`
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

type RESTServerData struct {
	Cert              string          `json:"cert,omitempty"`
	Key               string          `json:"key,omitempty"`
	Port              int             `json:"port,omitempty"`
	ListenAddr        string          `json:"listen_addr,omitempty"`
	WeatherSiteConfig WeatherSiteData `json:"weather_site"`
}

type WeatherSiteData struct {
	StationName      string  `json:"station_name,omitempty"`
	PullFromDevice   string  `json:"pull_from_device,omitempty"`
	SnowEnabled      bool    `json:"snow_enabled,omitempty"`
	SnowDevice       string  `json:"snow_device,omitempty"`
	SnowBaseDistance float32 `json:"snow_base_distance,omitempty"`
	PageTitle        string  `json:"page_title,omitempty"`
	AboutStationHTML string  `json:"about_station_html,omitempty"`
}

type ManagementAPIData struct {
	Cert       string `json:"cert,omitempty"`
	Key        string `json:"key,omitempty"`
	Port       int    `json:"port,omitempty"`
	ListenAddr string `json:"listen_addr,omitempty"`
	AuthToken  string `json:"auth_token,omitempty"`
	EnableCORS bool   `json:"enable_cors,omitempty"`
}
