package main

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// PWSWeatherController holds our connection along with some mutexes for operation
type PWSWeatherController struct {
	ctx    context.Context
	wg     *sync.WaitGroup
	Name   string `json:"name"`
	Config Config
	Logger *zap.SugaredLogger
}

// PWSWeatherConfig holds configuration for this controller
type PWSWeatherConfig struct {
	Name      string `yaml:"name"`
	StationID string `yaml:"station-id,omitempty"`
	APIKey    string `yaml:"api-key,omitempty"`
}
