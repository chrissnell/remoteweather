package snow

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SnowEvent represents a classified snow depth event for visualization
type SnowEvent struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Type         string    `json:"type"` // "accumulation", "plateau", "redistribution", "spike_then_settle"
	StartDepth   float64   `json:"start_depth_mm"`
	EndDepth     float64   `json:"end_depth_mm"`
	Accumulation float64   `json:"accumulation_mm,omitempty"`
}

// CachedSnowEvent represents a snow event record in the cache table
type CachedSnowEvent struct {
	StationName    string
	Hours          int
	StartTime      time.Time
	EndTime        time.Time
	EventType      string
	StartDepthMM   float64
	EndDepthMM     float64
	AccumulationMM sql.NullFloat64
	ComputedAt     time.Time
}

// CacheEventsForTimeRanges calculates and caches snow events for standard time ranges
// Called every 15 minutes by the snow cache controller
// Caches all events that contribute to snow totals (accumulation and spike_then_settle)
func (c *Calculator) CacheEventsForTimeRanges(ctx context.Context) error {
	// Standard time ranges to cache (in hours) - matches frontend CHART_RANGES
	timeRanges := []int{24, 72, 168, 744} // 24h, 72h, 7d, 30d

	for _, hours := range timeRanges {
		if err := c.cacheEventsForHours(ctx, hours); err != nil {
			c.logger.Errorf("Failed to cache events for %dh: %v", hours, err)
			// Continue with other time ranges even if one fails
			continue
		}
	}

	return nil
}

// cacheEventsForHours calculates and caches events for a specific time window
func (c *Calculator) cacheEventsForHours(ctx context.Context, hours int) error {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("cacheEventsForHours panic recovered (%dh): %v", hours, r)
		}
	}()

	// Add timeout protection
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Determine table based on time range
	var tableName string
	var days int

	if hours <= 168 { // 7 days or less
		tableName = "weather_1h"
		days = (hours / 24) + 1
	} else {
		tableName = "weather_1d"
		days = (hours / 24) + 1
	}

	// Fetch data
	readings, err := c.fetchData(ctx, tableName, days)
	if err != nil {
		return fmt.Errorf("fetch data failed: %w", err)
	}

	if len(readings) == 0 {
		c.logger.Debugf("No data available for %dh events", hours)
		return nil
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

	// Filter for events that contribute snow (accumulation and spike_then_settle)
	// These are the events with snowMM > 0 that should appear on charts
	var snowEvents []Segment
	for _, seg := range segments {
		if seg.SnowMM > 0 {
			snowEvents = append(snowEvents, seg)
		}
	}

	// Delete old cache entries for this time range
	deleteQuery := `DELETE FROM snow_events_cache WHERE stationname = $1 AND hours = $2`
	if _, err := c.db.ExecContext(ctx, deleteQuery, c.stationName, hours); err != nil {
		return fmt.Errorf("failed to delete old cache: %w", err)
	}

	// Insert new events into cache
	if len(snowEvents) > 0 {
		insertQuery := `
			INSERT INTO snow_events_cache
			(stationname, hours, start_time, end_time, event_type, start_depth_mm, end_depth_mm, accumulation_mm, computed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		`

		for _, event := range snowEvents {
			_, err := c.db.ExecContext(ctx, insertQuery,
				c.stationName,
				hours,
				event.StartTime,
				event.EndTime,
				event.Type,
				event.StartDepthMM,
				event.EndDepthMM,
				event.SnowMM,
			)
			if err != nil {
				c.logger.Errorf("Failed to insert event into cache: %v", err)
				// Continue with other events
				continue
			}
		}

		c.logger.Debugf("Cached %d snow events for %dh window (types: accumulation, spike_then_settle)", len(snowEvents), hours)
	}

	return nil
}

// GetSnowEvents returns cached snow events for a time window
// Includes both accumulation and spike_then_settle events (all events with snowMM > 0)
// Reads from snow_events_cache table populated every 15 minutes
func (c *Calculator) GetSnowEvents(ctx context.Context, hours int) ([]SnowEvent, error) {
	// Query cache for events
	// Accept cache entries up to 20 minutes old (gives 5 min grace period beyond 15 min refresh)
	query := `
		SELECT start_time, end_time, event_type, start_depth_mm, end_depth_mm, accumulation_mm
		FROM snow_events_cache
		WHERE stationname = $1
		  AND hours = $2
		  AND computed_at >= NOW() - INTERVAL '20 minutes'
		ORDER BY start_time
	`

	rows, err := c.db.QueryContext(ctx, query, c.stationName, hours)
	if err != nil {
		return nil, fmt.Errorf("query cache failed: %w", err)
	}
	defer rows.Close()

	var events []SnowEvent
	for rows.Next() {
		var event SnowEvent
		var accum sql.NullFloat64

		err := rows.Scan(
			&event.StartTime,
			&event.EndTime,
			&event.Type,
			&event.StartDepth,
			&event.EndDepth,
			&accum,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		if accum.Valid {
			event.Accumulation = accum.Float64
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return events, nil
}
