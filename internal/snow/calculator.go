// Package snow provides snowfall calculation and caching using pluggable computation strategies.
// Supports multiple algorithms (PELT, SQL, Smoothed) for 24h/72h/seasonal snowfall calculations.
package snow

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// Calculator provides snowfall calculations using a pluggable computation strategy.
// The strategy can be PELT-based, SQL-based, or any other implementation of SnowfallComputer.
type Calculator struct {
	computer     SnowfallComputer
	db           *sql.DB
	logger       *zap.SugaredLogger
	stationName  string
	baseDistance float64
}

// NewCalculator creates a Calculator with the specified computation strategy.
// computerType determines which algorithm is used (pelt, sql, etc.)
func NewCalculator(db *sql.DB, logger *zap.SugaredLogger, station string, baseDistance float64, computerType ComputerType) *Calculator {
	var computer SnowfallComputer

	switch computerType {
	case ComputerTypePELT:
		computer = NewPELTComputer(db, logger, station, baseDistance)
	case ComputerTypeSQL:
		computer = NewSQLComputer(db, logger, station, baseDistance)
	case ComputerTypeSmoothed:
		computer = NewSmoothedComputer(db, logger, station, baseDistance)
	default:
		// Default to PELT
		computer = NewPELTComputer(db, logger, station, baseDistance)
	}

	return &Calculator{
		computer:     computer,
		db:           db,
		logger:       logger,
		stationName:  station,
		baseDistance: baseDistance,
	}
}

// Calculate24h computes 24-hour snowfall accumulation
func (c *Calculator) Calculate24h(ctx context.Context) (float64, error) {
	return c.computer.Compute24h(ctx)
}

// Calculate72h computes 72-hour snowfall accumulation
func (c *Calculator) Calculate72h(ctx context.Context) (float64, error) {
	return c.computer.Compute72h(ctx)
}

// CalculateSeasonal computes seasonal snowfall accumulation
func (c *Calculator) CalculateSeasonal(ctx context.Context) (float64, error) {
	return c.computer.ComputeSeasonal(ctx)
}

// SetComputer allows runtime switching of computation strategy
func (c *Calculator) SetComputer(computer SnowfallComputer) {
	c.computer = computer
}
