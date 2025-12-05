package snow

import (
	"context"
	"database/sql"
	"fmt"
)

// SnowCache represents the current cached snow values
type SnowCache struct {
	Snow72h    float64
	SnowSeason float64
}

// RefreshCache updates all snow calculations and writes to cache
// This method implements the hybrid approach: SQL for midnight/24h, PELT for 72h/seasonal
// If PELT calculations fail, previous cached values are retained for 72h/seasonal
func (c *Calculator) RefreshCache(ctx context.Context) error {
	// SQL functions for fast calculations (midnight and 24h)
	midnight, err := c.getMidnight(ctx)
	if err != nil {
		return fmt.Errorf("midnight calculation failed: %w", err)
	}

	snow24h, err := c.get24h(ctx)
	if err != nil {
		return fmt.Errorf("24h calculation failed: %w", err)
	}

	// Get current cached values for graceful degradation
	currentCache, err := c.getCurrentCache(ctx)
	if err != nil {
		c.logger.Debugf("Unable to get current cache (may be first run): %v", err)
	}

	// PELT calculator for accurate multi-day calculations
	// On failure, keep previous values
	snow72h, err := c.Calculate72h(ctx)
	if err != nil {
		c.logger.Warnf("72h PELT calculation failed, keeping previous value: %v", err)
		if currentCache != nil {
			snow72h = currentCache.Snow72h
		} else {
			return fmt.Errorf("72h PELT calculation failed and no cached value available: %w", err)
		}
	}

	seasonal, err := c.CalculateSeasonal(ctx)
	if err != nil {
		c.logger.Warnf("Seasonal PELT calculation failed, keeping previous value: %v", err)
		if currentCache != nil {
			seasonal = currentCache.SnowSeason
		} else {
			return fmt.Errorf("seasonal PELT calculation failed and no cached value available: %w", err)
		}
	}

	// Update cache with all four values
	if err := c.updateCache(ctx, midnight, snow24h, snow72h, seasonal); err != nil {
		return err
	}

	// Log successful refresh with all values (debug level)
	c.logger.Debugf("Snow cache refreshed: midnight=%.1fmm, 24h=%.1fmm, 72h=%.1fmm (PELT), season=%.1fmm (PELT)",
		midnight, snow24h, snow72h, seasonal)

	return nil
}

// getMidnight calls the existing SQL function for midnight snow calculation
func (c *Calculator) getMidnight(ctx context.Context) (float64, error) {
	var snowMM sql.NullFloat64
	query := `SELECT get_new_snow_since_midnight($1, $2)`
	err := c.db.QueryRowContext(ctx, query, c.stationName, c.baseDistance).Scan(&snowMM)
	if err != nil {
		return 0, err
	}
	if !snowMM.Valid {
		return 0, nil
	}
	return snowMM.Float64, nil
}

// get24h calls the existing SQL function for 24-hour snow calculation
func (c *Calculator) get24h(ctx context.Context) (float64, error) {
	var snowMM sql.NullFloat64
	query := `SELECT get_new_snow_24h($1, $2)`
	err := c.db.QueryRowContext(ctx, query, c.stationName, c.baseDistance).Scan(&snowMM)
	if err != nil {
		return 0, err
	}
	if !snowMM.Valid {
		return 0, nil
	}
	return snowMM.Float64, nil
}

// getCurrentCache retrieves the current cached values for graceful degradation
func (c *Calculator) getCurrentCache(ctx context.Context) (*SnowCache, error) {
	query := `
		SELECT snow_72h, snow_season
		FROM snow_totals_cache
		WHERE stationname = $1
	`

	var cache SnowCache
	err := c.db.QueryRowContext(ctx, query, c.stationName).Scan(&cache.Snow72h, &cache.SnowSeason)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No cache exists yet, not an error
		}
		return nil, err
	}

	return &cache, nil
}

// updateCache performs an UPSERT on the snow_totals_cache table
func (c *Calculator) updateCache(ctx context.Context, midnight, snow24h, snow72h, seasonal float64) error {
	query := `
		INSERT INTO snow_totals_cache (
			stationname,
			snow_midnight,
			snow_24h,
			snow_72h,
			snow_season,
			computed_at
		) VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (stationname)
		DO UPDATE SET
			snow_midnight = EXCLUDED.snow_midnight,
			snow_24h = EXCLUDED.snow_24h,
			snow_72h = EXCLUDED.snow_72h,
			snow_season = EXCLUDED.snow_season,
			computed_at = EXCLUDED.computed_at
	`

	_, err := c.db.ExecContext(ctx, query, c.stationName, midnight, snow24h, snow72h, seasonal)
	if err != nil {
		return fmt.Errorf("cache update failed: %w", err)
	}

	return nil
}
