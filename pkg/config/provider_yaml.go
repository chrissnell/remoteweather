package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// YAMLProvider implements ConfigProvider for YAML configuration files
type YAMLProvider struct {
	filename string
	config   *ConfigData
}

// NewYAMLProvider creates a new YAML configuration provider
func NewYAMLProvider(filename string) *YAMLProvider {
	return &YAMLProvider{
		filename: filename,
	}
}

// LoadConfig loads the complete configuration from YAML file
func (y *YAMLProvider) LoadConfig() (*ConfigData, error) {
	cfgFile, err := os.ReadFile(y.filename)
	if err != nil {
		return nil, err
	}

	// Load into temporary struct with YAML tags
	var yamlConfig struct {
		Devices     []DeviceYAML     `yaml:"devices"`
		Storage     StorageYAML      `yaml:"storage,omitempty"`
		Controllers []ControllerYAML `yaml:"controllers,omitempty"`
	}

	err = yaml.Unmarshal(cfgFile, &yamlConfig)
	if err != nil {
		return nil, err
	}

	// Convert to our internal format
	config := &ConfigData{
		Devices:     make([]DeviceData, len(yamlConfig.Devices)),
		Controllers: make([]ControllerData, len(yamlConfig.Controllers)),
	}

	// Convert devices
	for i, device := range yamlConfig.Devices {
		config.Devices[i] = DeviceData{
			Name:              device.Name,
			Type:              device.Type,
			Hostname:          device.Hostname,
			Port:              device.Port,
			SerialDevice:      device.SerialDevice,
			Baud:              device.Baud,
			WindDirCorrection: device.WindDirCorrection,
			BaseSnowDistance:  device.BaseSnowDistance,
			Solar: SolarData{
				Latitude:  device.Solar.Latitude,
				Longitude: device.Solar.Longitude,
				Altitude:  device.Solar.Altitude,
			},
		}
	}

	// Convert storage
	config.Storage = StorageData{}
	if yamlConfig.Storage.TimescaleDB != nil {
		config.Storage.TimescaleDB = &TimescaleDBData{
			ConnectionString: yamlConfig.Storage.TimescaleDB.ConnectionString,
		}
	}
	if yamlConfig.Storage.GRPC != nil {
		config.Storage.GRPC = &GRPCData{
			Cert:           yamlConfig.Storage.GRPC.Cert,
			Key:            yamlConfig.Storage.GRPC.Key,
			ListenAddr:     yamlConfig.Storage.GRPC.ListenAddr,
			Port:           yamlConfig.Storage.GRPC.Port,
			PullFromDevice: yamlConfig.Storage.GRPC.PullFromDevice,
		}
	}
	if yamlConfig.Storage.APRS != nil {
		config.Storage.APRS = &APRSData{
			Callsign:     yamlConfig.Storage.APRS.Callsign,
			Passcode:     yamlConfig.Storage.APRS.Passcode,
			APRSISServer: yamlConfig.Storage.APRS.APRSISServer,
			Location: PointData{
				Lat: yamlConfig.Storage.APRS.Location.Lat,
				Lon: yamlConfig.Storage.APRS.Location.Lon,
			},
		}
	}

	// Convert controllers
	for i, controller := range yamlConfig.Controllers {
		config.Controllers[i] = ControllerData{
			Type: controller.Type,
		}

		if controller.PWSWeather != nil {
			config.Controllers[i].PWSWeather = &PWSWeatherData{
				StationID:      controller.PWSWeather.StationID,
				APIKey:         controller.PWSWeather.APIKey,
				APIEndpoint:    controller.PWSWeather.APIEndpoint,
				UploadInterval: controller.PWSWeather.UploadInterval,
				PullFromDevice: controller.PWSWeather.PullFromDevice,
			}
		}

		if controller.WeatherUnderground != nil {
			config.Controllers[i].WeatherUnderground = &WeatherUndergroundData{
				StationID:      controller.WeatherUnderground.StationID,
				APIKey:         controller.WeatherUnderground.APIKey,
				UploadInterval: controller.WeatherUnderground.UploadInterval,
				PullFromDevice: controller.WeatherUnderground.PullFromDevice,
				APIEndpoint:    controller.WeatherUnderground.APIEndpoint,
			}
		}

		if controller.AerisWeather != nil {
			config.Controllers[i].AerisWeather = &AerisWeatherData{
				APIClientID:     controller.AerisWeather.APIClientID,
				APIClientSecret: controller.AerisWeather.APIClientSecret,
				APIEndpoint:     controller.AerisWeather.APIEndpoint,
				Location:        controller.AerisWeather.Location,
			}
		}

		if controller.RESTServer != nil {
			config.Controllers[i].RESTServer = &RESTServerData{
				Cert:       controller.RESTServer.Cert,
				Key:        controller.RESTServer.Key,
				Port:       controller.RESTServer.Port,
				ListenAddr: controller.RESTServer.ListenAddr,
				WeatherSiteConfig: WeatherSiteData{
					StationName:      controller.RESTServer.WeatherSiteConfig.StationName,
					PullFromDevice:   controller.RESTServer.WeatherSiteConfig.PullFromDevice,
					SnowEnabled:      controller.RESTServer.WeatherSiteConfig.SnowEnabled,
					SnowDevice:       controller.RESTServer.WeatherSiteConfig.SnowDevice,
					SnowBaseDistance: controller.RESTServer.WeatherSiteConfig.SnowBaseDistance,
					PageTitle:        controller.RESTServer.WeatherSiteConfig.PageTitle,
					AboutStationHTML: controller.RESTServer.WeatherSiteConfig.AboutStationHTML,
				},
			}
		}

		if controller.ManagementAPI != nil {
			config.Controllers[i].ManagementAPI = &ManagementAPIData{
				Cert:       controller.ManagementAPI.Cert,
				Key:        controller.ManagementAPI.Key,
				Port:       controller.ManagementAPI.Port,
				ListenAddr: controller.ManagementAPI.ListenAddr,
				AuthToken:  controller.ManagementAPI.AuthToken,
				EnableCORS: controller.ManagementAPI.EnableCORS,
			}
		}
	}

	y.config = config
	return config, nil
}

// GetDevices returns device configurations
func (y *YAMLProvider) GetDevices() ([]DeviceData, error) {
	if y.config == nil {
		_, err := y.LoadConfig()
		if err != nil {
			return nil, err
		}
	}
	return y.config.Devices, nil
}

// GetStorageConfig returns storage configuration
func (y *YAMLProvider) GetStorageConfig() (*StorageData, error) {
	if y.config == nil {
		_, err := y.LoadConfig()
		if err != nil {
			return nil, err
		}
	}
	return &y.config.Storage, nil
}

// GetControllers returns controller configurations
func (y *YAMLProvider) GetControllers() ([]ControllerData, error) {
	if y.config == nil {
		_, err := y.LoadConfig()
		if err != nil {
			return nil, err
		}
	}
	return y.config.Controllers, nil
}

// IsReadOnly returns true since YAML files are read-only through this interface
func (y *YAMLProvider) IsReadOnly() bool {
	return true
}

// Close is a no-op for YAML provider
func (y *YAMLProvider) Close() error {
	return nil
}

// YAML-specific structs with proper YAML tags for parsing the original format
type DeviceYAML struct {
	Name              string    `yaml:"name"`
	Type              string    `yaml:"type,omitempty"`
	Hostname          string    `yaml:"hostname,omitempty"`
	Port              string    `yaml:"port,omitempty"`
	SerialDevice      string    `yaml:"serialdevice,omitempty"`
	Baud              int       `yaml:"baud,omitempty"`
	WindDirCorrection int16     `yaml:"wind-dir-correction,omitempty"`
	BaseSnowDistance  int16     `yaml:"base-snow-distance,omitempty"`
	Solar             SolarYAML `yaml:"solar,omitempty"`
}

type SolarYAML struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Altitude  float64 `yaml:"altitude"`
}

type StorageYAML struct {
	TimescaleDB *TimescaleDBYAML `yaml:"timescaledb,omitempty"`
	GRPC        *GRPCYAML        `yaml:"grpc,omitempty"`
	APRS        *APRSYAML        `yaml:"aprs,omitempty"`
}

type TimescaleDBYAML struct {
	ConnectionString string `yaml:"connection-string"`
}

type GRPCYAML struct {
	Cert           string `yaml:"cert,omitempty"`
	Key            string `yaml:"key,omitempty"`
	ListenAddr     string `yaml:"listen-addr,omitempty"`
	Port           int    `yaml:"port,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

type APRSYAML struct {
	Callsign     string    `yaml:"callsign,omitempty"`
	Passcode     string    `yaml:"passcode,omitempty"`
	APRSISServer string    `yaml:"aprs-is-server,omitempty"`
	Location     PointYAML `yaml:"location,omitempty"`
}

type PointYAML struct {
	Lat float64 `yaml:"latitude,omitempty"`
	Lon float64 `yaml:"longitude,omitempty"`
}

type ControllerYAML struct {
	Type               string                  `yaml:"type,omitempty"`
	PWSWeather         *PWSWeatherYAML         `yaml:"pwsweather,omitempty"`
	WeatherUnderground *WeatherUndergroundYAML `yaml:"weatherunderground,omitempty"`
	AerisWeather       *AerisWeatherYAML       `yaml:"aerisweather,omitempty"`
	RESTServer         *RESTServerYAML         `yaml:"rest,omitempty"`
	ManagementAPI      *ManagementAPIYAML      `yaml:"management,omitempty"`
}

type PWSWeatherYAML struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	APIEndpoint    string `yaml:"api-endpoint,omitempty"`
	UploadInterval string `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

type WeatherUndergroundYAML struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	UploadInterval string `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
	APIEndpoint    string `yaml:"api-endpoint,omitempty"`
}

type AerisWeatherYAML struct {
	APIClientID     string `yaml:"api-client-id"`
	APIClientSecret string `yaml:"api-client-secret"`
	APIEndpoint     string `yaml:"api-endpoint,omitempty"`
	Location        string `yaml:"location"`
}

type RESTServerYAML struct {
	Cert              string          `yaml:"cert,omitempty"`
	Key               string          `yaml:"key,omitempty"`
	Port              int             `yaml:"port,omitempty"`
	ListenAddr        string          `yaml:"listen-addr,omitempty"`
	WeatherSiteConfig WeatherSiteYAML `yaml:"weather-site"`
}

type WeatherSiteYAML struct {
	StationName      string  `yaml:"station-name,omitempty"`
	PullFromDevice   string  `yaml:"pull-from-device,omitempty"`
	SnowEnabled      bool    `yaml:"snow-enabled,omitempty"`
	SnowDevice       string  `yaml:"snow-device-name,omitempty"`
	SnowBaseDistance float32 `yaml:"snow-base-distance,omitempty"`
	PageTitle        string  `yaml:"page-title,omitempty"`
	AboutStationHTML string  `yaml:"about-station-html,omitempty"`
}

type ManagementAPIYAML struct {
	Cert       string `yaml:"cert,omitempty"`
	Key        string `yaml:"key,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	ListenAddr string `yaml:"listen-addr,omitempty"`
	AuthToken  string `yaml:"auth-token,omitempty"`
	EnableCORS bool   `yaml:"enable-cors,omitempty"`
}
