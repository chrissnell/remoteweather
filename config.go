package main

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Device DeviceConfig `yaml:"device"`
}

type DeviceConfig struct {
	Hostname string `yaml:"hostname"`
	Port     string `yaml:"port"`
}

// New creates an new config object from the given filename.
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
