package snow

import (
	"context"
)

// SnowfallComputer defines the interface for snowfall computation strategies.
type SnowfallComputer interface {
	Compute24h(ctx context.Context) (float64, error)
	Compute72h(ctx context.Context) (float64, error)
	ComputeSeasonal(ctx context.Context) (float64, error)
}

// ComputerType identifies the computation strategy
type ComputerType string

const (
	// ComputerTypePELT uses PELT changepoint detection (used for event caching)
	ComputerTypePELT ComputerType = "pelt"

	// ComputerTypeSmoothed uses quantile smoothing + rate limiting (default for totals)
	ComputerTypeSmoothed ComputerType = "smoothed"
)
