package main

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Controllers are components that run in a loop in the background and do something.  They might
// fetch data from an external source (e.g. a weather radar image from a commercial weather service)
// or send some periodic data to an outside service (e.g. PWS Weather).  They are different from
// weatherstations and storage backends in that they do not directly generate or process real-time
// weather readings.

// ControllerManager holds our active controller backends.
type ControllerManager struct {
	Controllers []Controller
}

// Controller is an interface that provides standard methods for various controller backends
type Controller interface {
	StartController() error
}

// NewControllerManager creats a ControllerManager object, populated with all configured
// controllers
func NewControllerManager(ctx context.Context, wg *sync.WaitGroup, c *Config, logger *zap.SugaredLogger) (*ControllerManager, error) {
	cm := ControllerManager{}
	// for _, c := range c.Controllers {

	// }

	return &cm, nil
}
