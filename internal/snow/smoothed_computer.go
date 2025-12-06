package snow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SmoothedComputer implements SnowfallComputer using pre-computed smoothed depth estimates.
// Queries the snow_depth_est_5m table which contains physically-plausible depth values
// computed via local quantile smoothing + rate limiting.
type SmoothedComputer struct {
	db           *sql.DB
	logger       *zap.SugaredLogger
	stationName  string
	baseDistance float64 // Kept for interface consistency, not used in queries
}

// NewSmoothedComputer creates a smoothed snowfall computer
func NewSmoothedComputer(db *sql.DB, logger *zap.SugaredLogger, station string, baseDistance float64) *SmoothedComputer {
	return &SmoothedComputer{
		db:           db,
		logger:       logger,
		stationName:  station,
		baseDistance: baseDistance,
	}
}

// Compute24h calculates 24-hour snowfall from estimated depth
// Returns the difference between max and min estimated depth in the last 24 hours (in mm)
func (s *SmoothedComputer) Compute24h(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("Smoothed calculator panic recovered (24h): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.computeAccumulation(ctx, 24*time.Hour)
}

// Compute72h calculates 72-hour snowfall from estimated depth
// Returns the difference between max and min estimated depth in the last 72 hours (in mm)
func (s *SmoothedComputer) Compute72h(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("Smoothed calculator panic recovered (72h): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.computeAccumulation(ctx, 72*time.Hour)
}

// ComputeSeasonal calculates seasonal snowfall from estimated depth
// Returns the difference between max depth since season start and depth at season start (in mm)
func (s *SmoothedComputer) ComputeSeasonal(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("Smoothed calculator panic recovered (seasonal): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get season start time
	seasonStart := getSeasonStart(time.Now())

	// Query for earliest time and max depth since season start
	var firstTime sql.NullTime
	var maxDepthIn sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT MIN(time), MAX(snow_depth_est_in)
		 FROM snow_depth_est_5m
		 WHERE stationname = $1
		   AND time >= $2`,
		s.stationName, seasonStart,
	).Scan(&firstTime, &maxDepthIn)

	if err == sql.ErrNoRows || !firstTime.Valid || !maxDepthIn.Valid {
		s.logger.Debugf("No seasonal data available for station %s", s.stationName)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query seasonal data: %w", err)
	}

	// Get depth at season start (or earliest available)
	var startDepthIn float64
	err = s.db.QueryRowContext(ctx,
		`SELECT snow_depth_est_in
		 FROM snow_depth_est_5m
		 WHERE stationname = $1
		   AND time >= $2
		 ORDER BY time ASC
		 LIMIT 1`,
		s.stationName, seasonStart,
	).Scan(&startDepthIn)

	if err == sql.ErrNoRows {
		s.logger.Debugf("No season start data for station %s", s.stationName)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query season start depth: %w", err)
	}

	// Calculate seasonal accumulation: max depth - start depth
	accumulationIn := maxDepthIn.Float64 - startDepthIn
	if accumulationIn < 0 {
		accumulationIn = 0
	}

	// Convert inches to mm
	accumulationMM := accumulationIn * 25.4

	s.logger.Debugf("Smoothed(seasonal) for %s: %.1fmm (%.1f\") from max %.1f\" - start %.1f\"",
		s.stationName, accumulationMM, accumulationIn, maxDepthIn.Float64, startDepthIn)

	return accumulationMM, nil
}

// computeAccumulation calculates snowfall for a given time window
// Returns max_depth - min_depth over the window (in mm)
func (s *SmoothedComputer) computeAccumulation(ctx context.Context, window time.Duration) (float64, error) {
	startTime := time.Now().Add(-window)

	var minDepthIn sql.NullFloat64
	var maxDepthIn sql.NullFloat64

	err := s.db.QueryRowContext(ctx,
		`SELECT MIN(snow_depth_est_in), MAX(snow_depth_est_in)
		 FROM snow_depth_est_5m
		 WHERE stationname = $1
		   AND time >= $2
		   AND time <= NOW()`,
		s.stationName, startTime,
	).Scan(&minDepthIn, &maxDepthIn)

	if err == sql.ErrNoRows || !minDepthIn.Valid || !maxDepthIn.Valid {
		s.logger.Debugf("No data available for station %s in window %v", s.stationName, window)
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query estimated depths: %w", err)
	}

	// Calculate accumulation: max - min
	accumulationIn := maxDepthIn.Float64 - minDepthIn.Float64
	if accumulationIn < 0 {
		accumulationIn = 0
	}

	// Convert inches to mm
	accumulationMM := accumulationIn * 25.4

	windowHours := int(window.Hours())
	s.logger.Debugf("Smoothed(%dh) for %s: %.1fmm (%.1f\") from max %.1f\" - min %.1f\"",
		windowHours, s.stationName, accumulationMM, accumulationIn, maxDepthIn.Float64, minDepthIn.Float64)

	return accumulationMM, nil
}
