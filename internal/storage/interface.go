// Package storage defines interfaces and implementations for weather data storage backends.
package storage

import (
	"context"
	"sync"

	"github.com/chrissnell/remoteweather/internal/types"
)

// StorageEngineInterface is an interface that provides a few standardized
// methods for various storage backends
type StorageEngineInterface interface {
	StartStorageEngine(context.Context, *sync.WaitGroup) chan<- types.Reading
}
