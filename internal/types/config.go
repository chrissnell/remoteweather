package types

import (
	htmltemplate "html/template"
	"os"

	"gopkg.in/yaml.v2"
)

// Config is the base configuration object
type Config struct {
	Devices     []DeviceConfig     `yaml:"devices"`
	Storage     StorageConfig      `yaml:"storage,omitempty"`
	Controllers []ControllerConfig `yaml:"controllers,omitempty"`
}

// DeviceConfig holds configuration specific to data collection devices
type DeviceConfig struct {
	Name              string      `yaml:"name"`
	Type              string      `yaml:"type,omitempty"`
	Hostname          string      `yaml:"hostname,omitempty"`
	Port              string      `yaml:"port,omitempty"`
	SerialDevice      string      `yaml:"serialdevice,omitempty"`
	Baud              int         `yaml:"baud,omitempty"`
	WindDirCorrection int16       `yaml:"wind-dir-correction,omitempty"`
	BaseSnowDistance  int16       `yaml:"base-snow-distance,omitempty"`
	Solar             SolarConfig `yaml:"solar,omitempty"`
}

// SolarConfig holds configuration specific to solar calculations
type SolarConfig struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Altitude  float64 `yaml:"altitude"`
}

// StorageConfig holds the configuration for various storage backends.
// More than one storage backend can be used simultaneously
type StorageConfig struct {
	InfluxDB    InfluxDBConfig    `yaml:"influxdb,omitempty"`
	TimescaleDB TimescaleDBConfig `yaml:"timescaledb,omitempty"`
	GRPC        GRPCConfig        `yaml:"grpc,omitempty"`
	APRS        APRSConfig        `yaml:"aprs,omitempty"`
}

// ControllerConfig holds the configuration for various controller backends.
// More than one controller backend can be used simultaneously.
type ControllerConfig struct {
	Type               string                   `yaml:"type,omitempty"`
	PWSWeather         PWSWeatherConfig         `yaml:"pwsweather,omitempty"`
	WeatherUnderground WeatherUndergroundConfig `yaml:"weatherunderground,omitempty"`
	AerisWeather       AerisWeatherConfig       `yaml:"aerisweather,omitempty"`
	RESTServer         RESTServerConfig         `yaml:"rest,omitempty"`
	ManagementAPI      ManagementAPIConfig      `yaml:"management,omitempty"`
}

// Storage backend configurations
type InfluxDBConfig struct {
	Scheme   string `yaml:"scheme,omitempty"`
	Host     string `yaml:"host,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Database string `yaml:"database,omitempty"`
	Protocol string `yaml:"protocol,omitempty"`
}

type TimescaleDBConfig struct {
	ConnectionString string `yaml:"connection-string,omitempty"`
}

type GRPCConfig struct {
	Cert           string `yaml:"cert,omitempty"`
	Key            string `yaml:"key,omitempty"`
	ListenAddr     string `yaml:"listen-addr,omitempty"`
	Port           int    `yaml:"port,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

type APRSConfig struct {
	Callsign     string `yaml:"callsign,omitempty"`
	Passcode     string `yaml:"passcode,omitempty"`
	APRSISServer string `yaml:"aprsis-server,omitempty"`
	Location     Point  `yaml:"location,omitempty"`
}

type Point struct {
	Lat float64 `yaml:"lat"`
	Lon float64 `yaml:"lon"`
}

// Controller backend configurations
type PWSWeatherConfig struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	APIEndpoint    string `yaml:"api-endpoint,omitempty"`
	UploadInterval string `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

type WeatherUndergroundConfig struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	UploadInterval string `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
	APIEndpoint    string `yaml:"api-endpoint,omitempty"`
}

type AerisWeatherConfig struct {
	APIClientID     string `yaml:"api-client-id,omitempty"`
	APIClientSecret string `yaml:"api-client-secret,omitempty"`
	APIEndpoint     string `yaml:"api-endpoint,omitempty"`
	Location        string `yaml:"location,omitempty"`
}

type RESTServerConfig struct {
	Cert              string            `yaml:"cert,omitempty"`
	Key               string            `yaml:"key,omitempty"`
	Port              int               `yaml:"port,omitempty"`
	ListenAddr        string            `yaml:"listen-addr,omitempty"`
	WeatherSiteConfig WeatherSiteConfig `yaml:"weather-site"`
}

type ManagementAPIConfig struct {
	Cert       string `yaml:"cert,omitempty"`
	Key        string `yaml:"key,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	ListenAddr string `yaml:"listen-addr,omitempty"`
	AuthToken  string `yaml:"auth-token,omitempty"`
	EnableCORS bool   `yaml:"enable-cors,omitempty"`
}

type WeatherSiteConfig struct {
	StationName      string            `yaml:"station-name,omitempty"`
	PullFromDevice   string            `yaml:"pull-from-device,omitempty"`
	SnowEnabled      bool              `yaml:"snow-enabled,omitempty"`
	SnowDevice       string            `yaml:"snow-device-name,omitempty"`
	SnowBaseDistance float32           `yaml:"snow-base-distance,omitempty"`
	PageTitle        string            `yaml:"page-title,omitempty"`
	AboutStationHTML htmltemplate.HTML `yaml:"about-station-html,omitempty"`
}

// NewConfig creates a new config object from the given filename.
func NewConfig(filename string) (Config, error) {
	cfgFile, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}
	c := Config{}
	err = yaml.Unmarshal(cfgFile, &c)
	if err != nil {
		return Config{}, err
	}
	return c, nil
}
