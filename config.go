package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config is the base configuraiton object
type Config struct {
	Device  DeviceConfig  `yaml:"device"`
	Storage StorageConfig `yaml:"storage,omitempty"`
}

// DeviceConfig holds configuration specific to the Davis Instruments device
type DeviceConfig struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Port     string `yaml:"port"`
}

// StorageConfig holds the configuration for various storage backends.
// More than one storage backend can be used simultaneously
type StorageConfig struct {
	InfluxDB InfluxDBConfig `yaml:"influxdb,omitempty"`
}

// NewConfig creates an new config object from the given filename.
func NewConfig(filename string) (Config, error) {
	cfgFile, err := ioutil.ReadFile(filename)
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
