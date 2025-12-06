package snow

import (
	"context"
)

// SnowfallComputer defines the interface for different snowfall computation strategies.
// Implementations can use PELT algorithms, SQL queries, or other methods.
// All implementations should compute snowfall accumulation for standard time periods.
type SnowfallComputer interface {
	// Compute24h calculates 24-hour snowfall accumulation in millimeters
	Compute24h(ctx context.Context) (float64, error)

	// Compute72h calculates 72-hour snowfall accumulation in millimeters
	Compute72h(ctx context.Context) (float64, error)

	// ComputeSeasonal calculates seasonal snowfall accumulation in millimeters
	ComputeSeasonal(ctx context.Context) (float64, error)
}

// ComputerType identifies the computation strategy
type ComputerType string

const (
	// ComputerTypePELT uses PELT changepoint detection with median filtering
	ComputerTypePELT ComputerType = "pelt"

	// ComputerTypeSQL uses PostgreSQL functions for computation
	ComputerTypeSQL ComputerType = "sql"
)
