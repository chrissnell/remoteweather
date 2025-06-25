package main

import "github.com/chrissnell/remoteweather/internal/types"

// Re-export types from the types package for backward compatibility
type Config = types.Config
type DeviceConfig = types.DeviceConfig
type SolarConfig = types.SolarConfig
type StorageConfig = types.StorageConfig
type ControllerConfig = types.ControllerConfig
type InfluxDBConfig = types.InfluxDBConfig
type TimescaleDBConfig = types.TimescaleDBConfig
type GRPCConfig = types.GRPCConfig
type APRSConfig = types.APRSConfig
type Point = types.Point
type PWSWeatherConfig = types.PWSWeatherConfig
type WeatherUndergroundConfig = types.WeatherUndergroundConfig
type AerisWeatherConfig = types.AerisWeatherConfig
type RESTServerConfig = types.RESTServerConfig
type WeatherSiteConfig = types.WeatherSiteConfig

// NewConfig creates a new config object from the given filename.
var NewConfig = types.NewConfig
