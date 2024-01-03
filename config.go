package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config is the base configuraiton object
type Config struct {
	Devices     []DeviceConfig     `yaml:"devices"`
	Storage     StorageConfig      `yaml:"storage,omitempty"`
	Controllers []ControllerConfig `yaml:"controllers,omitempty"`
}

// DeviceConfig holds configuration specific to the Davis Instruments device
type DeviceConfig struct {
	Name         string `yaml:"name"`
	Type         string `yaml:"type,omitempty"`
	Hostname     string `yaml:"hostname,omitempty"`
	Port         string `yaml:"port,omitempty"`
	SerialDevice string `yaml:"serialdevice,omitempty"`
}

// StorageConfig holds the configuration for various storage backends.
// More than one storage backend can be used simultaneously
type StorageConfig struct {
	InfluxDB    InfluxDBConfig    `yaml:"influxdb,omitempty"`
	TimescaleDB TimescaleDBConfig `yaml:"timescaledb,omitempty"`
	GRPC        GRPCConfig        `yaml:"grpc,omitempty"`
	RESTServer  RESTServerConfig  `yaml:"rest,omitempty"`
	APRS        APRSConfig        `yaml:"aprs,omitempty"`
	WU          WUConfig          `yaml:"wunderground,omitempty"`
}

// ControllerConfig holds the configuration for various controller backends.
// More than one controller backend can be used simultaneously.
type ControllerConfig struct {
	Type       string           `yaml:"type,omitempty"`
	PWSWeather PWSWeatherConfig `yaml:"pwsweather,omitempty"`
}

// NewConfig creates an new config object from the given filename.
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
