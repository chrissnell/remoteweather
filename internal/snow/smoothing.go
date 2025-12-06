package snow

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"
)

// SmoothingParams defines parameters for the quantile smoothing + rate limiting algorithm
type SmoothingParams struct {
	// WindowMinutes is the half-window size for local quantile smoothing (e.g., 30)
	// A window of 30 minutes means we look at ±30 minutes around each point
	WindowMinutes int

	// Quantile is the upper quantile to use for smoothing (0-1, e.g., 0.85 or 0.9)
	// 0.85 means we take the 85th percentile of depths in the local window
	Quantile float64

	// MaxUpRateInPerHour is the maximum allowed snow accumulation rate in inches/hour (e.g., 4.0)
	MaxUpRateInPerHour float64

	// MaxDownRateInPerHour is the maximum allowed snow settling/melt rate in inches/hour (e.g., 1.5)
	MaxDownRateInPerHour float64
}

// DefaultSmoothingParams returns conservative default parameters for smoothing
func DefaultSmoothingParams() SmoothingParams {
	return SmoothingParams{
		WindowMinutes:        30,   // ±30 minutes = 1-hour window
		Quantile:             0.85, // 85th percentile
		MaxUpRateInPerHour:   4.0,  // Max 4 inches/hour accumulation
		MaxDownRateInPerHour: 1.5,  // Max 1.5 inches/hour settling
	}
}

// Sample represents a single time/depth measurement in inches
type Sample struct {
	Time    time.Time
	DepthIn float64
}

// LocalQuantileSmooth applies local upper-quantile smoothing to a time series
// For each point, it computes the specified quantile of all points within the time window
func LocalQuantileSmooth(samples []Sample, params SmoothingParams) []float64 {
	n := len(samples)
	if n == 0 {
		return []float64{}
	}

	smoothed := make([]float64, n)
	windowDuration := time.Duration(params.WindowMinutes) * time.Minute

	for i := 0; i < n; i++ {
		currentTime := samples[i].Time
		windowStart := currentTime.Add(-windowDuration)
		windowEnd := currentTime.Add(windowDuration)

		// Collect all depths within the time window
		var windowDepths []float64
		for j := 0; j < n; j++ {
			if !samples[j].Time.Before(windowStart) && !samples[j].Time.After(windowEnd) {
				windowDepths = append(windowDepths, samples[j].DepthIn)
			}
		}

		if len(windowDepths) == 0 {
			// Shouldn't happen, but use original value as fallback
			smoothed[i] = samples[i].DepthIn
			continue
		}

		// Sort depths to compute quantile
		sort.Float64s(windowDepths)

		// Compute quantile index
		quantileIdx := int(math.Floor(params.Quantile * float64(len(windowDepths)-1)))

		// Optionally average a small band around the quantile for extra stability
		// We'll average 3 points centered on the quantile index
		lowIdx := quantileIdx
		highIdx := quantileIdx + 2
		if highIdx > len(windowDepths)-1 {
			highIdx = len(windowDepths) - 1
		}
		if highIdx-lowIdx < 2 && lowIdx > 0 {
			newLowIdx := highIdx - 2
			if newLowIdx < 0 {
				newLowIdx = 0
			}
			lowIdx = newLowIdx
		}

		// Compute mean of the band
		sum := 0.0
		count := 0
		for k := lowIdx; k <= highIdx; k++ {
			sum += windowDepths[k]
			count++
		}

		smoothed[i] = sum / float64(count)
	}

	return smoothed
}

// ApplyRateLimiting applies rate limits to a smoothed depth series
// This ensures depth changes don't exceed physically plausible accumulation/settling rates
func ApplyRateLimiting(samples []Sample, smoothedDepths []float64, params SmoothingParams, prevEstimate *Sample) []float64 {
	n := len(samples)
	if n == 0 {
		return []float64{}
	}

	limited := make([]float64, n)

	// Initialize with previous estimate or first smoothed value
	var prevDepth float64
	var prevTime time.Time
	if prevEstimate != nil {
		prevDepth = prevEstimate.DepthIn
		prevTime = prevEstimate.Time
	} else {
		prevDepth = smoothedDepths[0]
		prevTime = samples[0].Time
		limited[0] = smoothedDepths[0]
		if n == 1 {
			return limited
		}
		// Start from index 1 if we used first value
		prevTime = samples[0].Time
		prevDepth = limited[0]
	}

	startIdx := 0
	if prevEstimate == nil {
		startIdx = 1
	}

	for i := startIdx; i < n; i++ {
		dtHours := samples[i].Time.Sub(prevTime).Hours()
		if dtHours <= 0 {
			// Time didn't advance, keep previous depth
			limited[i] = prevDepth
			continue
		}

		rawDelta := smoothedDepths[i] - prevDepth
		rawRate := rawDelta / dtHours

		// Apply rate limits
		cappedRate := rawRate
		if rawRate > params.MaxUpRateInPerHour {
			cappedRate = params.MaxUpRateInPerHour
		} else if rawRate < -params.MaxDownRateInPerHour {
			cappedRate = -params.MaxDownRateInPerHour
		}

		limited[i] = prevDepth + cappedRate*dtHours

		// Update for next iteration
		prevDepth = limited[i]
		prevTime = samples[i].Time
	}

	return limited
}

// UpdateSnowDepthEstimates computes and stores estimated snow depth for a station
// This function:
// 1. Fetches raw snowdistance data from weather_5m
// 2. Converts to depth in inches
// 3. Applies local quantile smoothing
// 4. Applies rate limiting
// 5. Stores results in snow_depth_est_5m
func UpdateSnowDepthEstimates(
	ctx context.Context,
	db *sql.DB,
	logger *zap.SugaredLogger,
	station string,
	baseDistanceMM float64,
	params SmoothingParams,
) error {
	// Find the last estimated time for this station
	var lastEstTime sql.NullTime
	err := db.QueryRowContext(ctx,
		`SELECT MAX(time) FROM snow_depth_est_5m WHERE stationname = $1`,
		station,
	).Scan(&lastEstTime)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get last estimate time: %w", err)
	}

	var startTime time.Time
	var prevEstimate *Sample

	if !lastEstTime.Valid {
		// Initial backfill: start from season start
		startTime = getSeasonStart(time.Now())
		logger.Infof("Initial backfill for %s starting from season start: %s", station, startTime.Format("2006-01-02"))
	} else {
		// Incremental update: overlap by 6 hours for smoothing boundary
		startTime = lastEstTime.Time.Add(-6 * time.Hour)

		// Get the previous estimate value from BEFORE startTime for rate limiting continuity
		var prevDepth float64
		var prevTime time.Time
		err = db.QueryRowContext(ctx,
			`SELECT time, snow_depth_est_in FROM snow_depth_est_5m
			 WHERE stationname = $1 AND time < $2
			 ORDER BY time DESC
			 LIMIT 1`,
			station, startTime,
		).Scan(&prevTime, &prevDepth)
		if err == nil {
			prevEstimate = &Sample{
				Time:    prevTime,
				DepthIn: prevDepth,
			}
		}
	}

	// Fetch raw data from weather_5m
	rows, err := db.QueryContext(ctx,
		`SELECT bucket, snowdistance
		 FROM weather_5m
		 WHERE stationname = $1
		   AND bucket >= $2
		   AND snowdistance IS NOT NULL
		   AND snowdistance < $3 - 2
		 ORDER BY bucket`,
		station, startTime, baseDistanceMM,
	)
	if err != nil {
		return fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer rows.Close()

	// Convert to depth samples in inches
	var samples []Sample
	for rows.Next() {
		var t time.Time
		var snowdistance float64
		if err := rows.Scan(&t, &snowdistance); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		depthMM := baseDistanceMM - snowdistance
		depthIn := depthMM / 25.4

		samples = append(samples, Sample{
			Time:    t,
			DepthIn: depthIn,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	if len(samples) == 0 {
		logger.Debugf("No data to process for %s from %s", station, startTime.Format("2006-01-02 15:04"))
		return nil
	}

	logger.Debugf("Processing %d samples for %s from %s to %s",
		len(samples), station,
		samples[0].Time.Format("2006-01-02 15:04"),
		samples[len(samples)-1].Time.Format("2006-01-02 15:04"))

	// Step 1: Apply local quantile smoothing
	smoothedDepths := LocalQuantileSmooth(samples, params)

	// Step 2: Apply rate limiting
	limitedDepths := ApplyRateLimiting(samples, smoothedDepths, params, prevEstimate)

	// Delete overlapping estimates
	_, err = db.ExecContext(ctx,
		`DELETE FROM snow_depth_est_5m WHERE stationname = $1 AND time >= $2`,
		station, startTime,
	)
	if err != nil {
		return fmt.Errorf("failed to delete overlapping estimates: %w", err)
	}

	// Bulk insert new estimates
	// Use a transaction for atomic insert
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO snow_depth_est_5m (stationname, time, snow_depth_est_in)
		 VALUES ($1, $2, $3)`,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for i, sample := range samples {
		_, err = stmt.ExecContext(ctx, station, sample.Time, limitedDepths[i])
		if err != nil {
			return fmt.Errorf("failed to insert estimate: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debugf("Inserted %d estimated depth values for %s", len(samples), station)
	return nil
}

// getSeasonStart returns the start of the current snow season (October 1)
func getSeasonStart(now time.Time) time.Time {
	year := now.Year()
	month := now.Month()

	// If we're before October, season started last year
	if month < time.October {
		year--
	}

	return time.Date(year, time.October, 1, 0, 0, 0, 0, now.Location())
}
