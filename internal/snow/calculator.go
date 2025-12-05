// Package snow provides PELT-based snow accumulation calculation using statistical change point detection.
// This package replaces traditional dual-threshold algorithms with more accurate change point analysis
// for 72-hour and seasonal snow accumulation calculations.
package snow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Calculator performs PELT-based snow accumulation calculations using signal processing
type Calculator struct {
	db              *sql.DB
	logger          *zap.SugaredLogger
	stationName     string
	baseDistance    float64
	smoothingWindow int
	penalty         float64
	minAccumulation float64
	minSize         int
	jump            int
}

// NewCalculator creates a new PELT-based snow calculator
// baseDistance is the calibration value for the ultrasonic sensor (mm from sensor to ground at 0 snow)
func NewCalculator(db *sql.DB, logger *zap.SugaredLogger, station string, baseDistance float64) *Calculator {
	return &Calculator{
		db:              db,
		logger:          logger,
		stationName:     station,
		baseDistance:    baseDistance,
		smoothingWindow: 5,    // 5-hour median filter
		penalty:         3.0,  // PELT penalty parameter
		minAccumulation: 5.0,  // 5mm minimum to count as accumulation
		minSize:         2,    // Minimum 2-hour segments
		jump:            1,    // No subsampling
	}
}

// Calculate72h computes 72-hour snow accumulation using PELT change point detection
// Includes timeout protection and panic recovery for graceful degradation
func (c *Calculator) Calculate72h(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("PELT calculator panic recovered (72h): %v", r)
		}
	}()

	// Add timeout for PELT calculation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.calculateAccumulation(ctx, "weather_1h", 3) // 3 days
	if err != nil {
		c.logger.Debugf("72h calculation error: %v", err)
		return 0, err
	}

	return result, nil
}

// CalculateSeasonal computes seasonal snow accumulation using PELT change point detection
// Includes timeout protection and panic recovery for graceful degradation
func (c *Calculator) CalculateSeasonal(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("PELT calculator panic recovered (seasonal): %v", r)
		}
	}()

	// Add timeout for PELT calculation
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use hourly data for recent season (30 days) to capture all snow including today
	// Daily aggregates (weather_1d) don't include today's snow until the day completes
	// For longer historical analysis, a hybrid approach with daily data could be added
	// but 30 days of hourly data gives good seasonal context while staying performant
	result, err := c.calculateAccumulation(ctx, "weather_1h", 30)
	if err != nil {
		c.logger.Debugf("Seasonal calculation error: %v", err)
		return 0, err
	}

	return result, nil
}

// calculateAccumulation performs the PELT calculation on the specified table and time range
func (c *Calculator) calculateAccumulation(ctx context.Context, tableName string, days int) (float64, error) {
	// Fetch data
	readings, err := c.fetchData(ctx, tableName, days)
	if err != nil {
		return 0, err
	}

	if len(readings) == 0 {
		return 0, fmt.Errorf("no data available for station %s", c.stationName)
	}

	// Extract depths
	depths := make([]float64, len(readings))
	for i, r := range readings {
		depths[i] = r.DepthMM
	}

	// Apply median smoothing
	smoothed := MedFilt(depths, c.smoothingWindow)

	// Detect changepoints using PELT
	detector := NewPeltDetector(c.minSize, c.jump)
	detector.Fit(smoothed)
	breakpoints := detector.Predict(c.penalty)

	// Classify segments
	segments := c.classifySegments(readings, smoothed, breakpoints)

	// Sum accumulation and log segments
	totalMM := 0.0
	accumulationEvents := 0
	for _, seg := range segments {
		if seg.SnowMM > 0 {
			c.logger.Debugf("  %s segment: %.1fmm snow (%s to %s)",
				seg.Type, seg.SnowMM,
				seg.StartTime.Format("01/02 15:04"),
				seg.EndTime.Format("01/02 15:04"))
			accumulationEvents++
		}
		totalMM += seg.SnowMM
	}

	c.logger.Debugf("PELT found %d accumulation events, total: %.1fmm (%.1f\") from %d segments over %d readings",
		accumulationEvents, totalMM, totalMM/25.4, len(segments), len(readings))

	return totalMM, nil
}

// fetchData retrieves snow depth data from the database
func (c *Calculator) fetchData(ctx context.Context, tableName string, days int) ([]SnowReading, error) {
	query := fmt.Sprintf(`
		SELECT
			bucket AT TIME ZONE 'America/Denver' as time_mst,
			$2 - snowdistance as depth_mm
		FROM %s
		WHERE stationname = $1
		  AND bucket >= NOW() - INTERVAL '1 day' * $3
		  AND snowdistance IS NOT NULL
		  AND snowdistance < $2 - 2
		ORDER BY bucket
	`, tableName)

	rows, err := c.db.QueryContext(ctx, query, c.stationName, c.baseDistance, days)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var readings []SnowReading
	for rows.Next() {
		var r SnowReading
		if err := rows.Scan(&r.Time, &r.DepthMM); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		readings = append(readings, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return readings, nil
}

// classifySegments categorizes each segment and calculates accumulation
func (c *Calculator) classifySegments(readings []SnowReading, smoothed []float64, breakpoints []int) []Segment {
	var segments []Segment

	// Ensure breakpoints include start
	allBreakpoints := []int{}
	if len(breakpoints) == 0 || breakpoints[0] != 0 {
		allBreakpoints = append(allBreakpoints, 0)
	}
	allBreakpoints = append(allBreakpoints, breakpoints...)

	for i := 0; i < len(allBreakpoints)-1; i++ {
		startIdx := allBreakpoints[i]
		endIdx := allBreakpoints[i+1]

		if endIdx > len(smoothed) {
			endIdx = len(smoothed)
		}

		segmentDepths := smoothed[startIdx:endIdx]
		if len(segmentDepths) == 0 {
			continue
		}

		startDepth := segmentDepths[0]
		endDepth := segmentDepths[len(segmentDepths)-1]
		maxDepth := max(segmentDepths)
		minDepth := min(segmentDepths)

		netChange := endDepth - startDepth
		maxIncrease := maxDepth - startDepth
		maxDecrease := startDepth - minDepth

		var segmentType string
		var snowMM float64

		// Classification logic from standalone script
		if maxIncrease >= c.minAccumulation {
			if netChange >= c.minAccumulation*0.5 {
				segmentType = "accumulation"
				snowMM = netChange
			} else {
				segmentType = "spike_then_settle"
				snowMM = maxIncrease
			}
		} else if maxDecrease >= c.minAccumulation {
			segmentType = "redistribution"
			snowMM = 0
		} else {
			segmentType = "plateau"
			snowMM = 0
		}

		segments = append(segments, Segment{
			StartTime:     readings[startIdx].Time,
			EndTime:       readings[endIdx-1].Time,
			StartDepthMM:  startDepth,
			EndDepthMM:    endDepth,
			MaxDepthMM:    maxDepth,
			MinDepthMM:    minDepth,
			DurationHours: endIdx - startIdx,
			NetChangeMM:   netChange,
			MaxIncreaseMM: maxIncrease,
			MaxDecreaseMM: maxDecrease,
			Type:          segmentType,
			SnowMM:        snowMM,
		})
	}

	return segments
}

// Helper functions
func max(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	maxVal := data[0]
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func min(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	minVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}
