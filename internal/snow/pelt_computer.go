package snow

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PELTComputer implements SnowfallComputer using PELT changepoint detection.
// Uses median filtering for noise reduction and PELT algorithm for segment detection.
type PELTComputer struct {
	db              *sql.DB
	logger          *zap.SugaredLogger
	stationName     string
	baseDistance    float64
	penalty         float64
	minAccumulation float64
	minSize         int
	jump            int
}

// NewPELTComputer creates a PELT-based snowfall computer with default parameters
func NewPELTComputer(db *sql.DB, logger *zap.SugaredLogger, station string, baseDistance float64) *PELTComputer {
	return &PELTComputer{
		db:              db,
		logger:          logger,
		stationName:     station,
		baseDistance:    baseDistance,
		penalty:         8.0,  // PELT penalty parameter (higher = fewer breakpoints)
		minAccumulation: 10.0, // 10mm minimum to count as accumulation
		minSize:         2,    // Minimum 2-hour segments
		jump:            1,    // No subsampling
	}
}

// Compute24h calculates 24-hour snowfall using PELT on 5-minute data
func (p *PELTComputer) Compute24h(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("PELT calculator panic recovered (24h): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := p.calculateAccumulation(ctx, "weather_5m", 1)
	if err != nil {
		p.logger.Debugf("24h calculation error: %v", err)
		return 0, err
	}

	return result, nil
}

// Compute72h calculates 72-hour snowfall using PELT on 5-minute data
func (p *PELTComputer) Compute72h(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("PELT calculator panic recovered (72h): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := p.calculateAccumulation(ctx, "weather_5m", 3)
	if err != nil {
		p.logger.Debugf("72h calculation error: %v", err)
		return 0, err
	}

	return result, nil
}

// ComputeSeasonal calculates seasonal snowfall using PELT on hourly data
func (p *PELTComputer) ComputeSeasonal(ctx context.Context) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Errorf("PELT calculator panic recovered (seasonal): %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := p.calculateAccumulation(ctx, "weather_1h", 30)
	if err != nil {
		p.logger.Debugf("Seasonal calculation error: %v", err)
		return 0, err
	}

	return result, nil
}

// calculateAccumulation performs PELT-based calculation on specified table and time range
func (p *PELTComputer) calculateAccumulation(ctx context.Context, tableName string, days int) (float64, error) {
	// Fetch data
	readings, err := p.fetchData(ctx, tableName, days)
	if err != nil {
		return 0, err
	}

	if len(readings) == 0 {
		return 0, fmt.Errorf("no data available for station %s", p.stationName)
	}

	p.logger.Debugf("PELT calculation starting: table=%s, days=%d, fetched %d readings", tableName, days, len(readings))

	// Extract depths
	depths := make([]float64, len(readings))
	for i, r := range readings {
		depths[i] = r.DepthMM
	}

	// Calculate smoothing window based on data resolution
	smoothingWindow := p.getSmoothingWindow(tableName)

	// Apply median smoothing
	smoothed := MedFilt(depths, smoothingWindow)
	p.logger.Debugf("Applied median filter: window=%d readings (table=%s)", smoothingWindow, tableName)

	// Detect changepoints using PELT
	detector := NewPeltDetector(p.minSize, p.jump)
	detector.Fit(smoothed)
	breakpoints := detector.Predict(p.penalty)

	// Classify segments
	segments := p.classifySegments(readings, smoothed, breakpoints)

	// Sum accumulation and log segments
	totalMM := 0.0
	accumulationEvents := 0
	for _, seg := range segments {
		if seg.SnowMM > 0 {
			p.logger.Debugf("  %s segment: %.1fmm snow (%s to %s)",
				seg.Type, seg.SnowMM,
				seg.StartTime.Format("01/02 15:04"),
				seg.EndTime.Format("01/02 15:04"))
			accumulationEvents++
		}
		totalMM += seg.SnowMM
	}

	p.logger.Debugf("PELT(%d days) found %d accumulation events, total: %.1fmm (%.1f\") from %d segments over %d readings",
		days, accumulationEvents, totalMM, totalMM/25.4, len(segments), len(readings))

	return totalMM, nil
}

// fetchData retrieves snow depth data from the database
func (p *PELTComputer) fetchData(ctx context.Context, tableName string, days int) ([]SnowReading, error) {
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

	rows, err := p.db.QueryContext(ctx, query, p.stationName, p.baseDistance, days)
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

// getSmoothingWindow returns appropriate window size for data resolution
func (p *PELTComputer) getSmoothingWindow(tableName string) int {
	switch tableName {
	case "weather_5m":
		return 61 // 61 * 5min = 305min ~5 hours (must be odd)
	case "weather_1h":
		return 5 // 5 * 1hour = 5 hours
	case "weather_1d":
		return 1 // Daily data doesn't need smoothing
	default:
		return 5 // Default fallback
	}
}

// classifySegments categorizes each segment and calculates accumulation
func (p *PELTComputer) classifySegments(readings []SnowReading, smoothed []float64, breakpoints []int) []Segment {
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

		// Classification logic
		if maxIncrease >= p.minAccumulation {
			if netChange >= p.minAccumulation*0.5 {
				segmentType = "accumulation"
				snowMM = netChange
			} else {
				segmentType = "spike_then_settle"
				snowMM = maxIncrease
			}
		} else if maxDecrease >= p.minAccumulation {
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

// Public methods for event caching

// FetchData retrieves snow depth data from the database (public for event caching)
func (p *PELTComputer) FetchData(ctx context.Context, tableName string, days int) ([]SnowReading, error) {
	return p.fetchData(ctx, tableName, days)
}

// ClassifySegments categorizes segments and calculates accumulation (public for event caching)
func (p *PELTComputer) ClassifySegments(readings []SnowReading, smoothed []float64, breakpoints []int) []Segment {
	return p.classifySegments(readings, smoothed, breakpoints)
}

// GetSmoothingWindow returns appropriate window size for data resolution (public for event caching)
func (p *PELTComputer) GetSmoothingWindow(tableName string) int {
	return p.getSmoothingWindow(tableName)
}

// GetPenalty returns the PELT penalty parameter
func (p *PELTComputer) GetPenalty() float64 {
	return p.penalty
}

// GetMinSize returns the minimum segment size
func (p *PELTComputer) GetMinSize() int {
	return p.minSize
}

// GetJump returns the jump parameter
func (p *PELTComputer) GetJump() int {
	return p.jump
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
