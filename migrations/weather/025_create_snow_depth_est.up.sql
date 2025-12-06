-- Migration 025: Create snow_depth_est_5m table for smoothed snowfall computation
--
-- Purpose: Store estimated snow depth using local upper-quantile smoothing + rate limiting
--
-- This table provides a physically-plausible snow depth time series that:
-- 1. Removes sensor noise and short-term fluctuations using quantile smoothing
-- 2. Enforces realistic accumulation/settling rates via rate limiting
-- 3. Serves as input for the SmoothedComputer snowfall calculation strategy
--
-- The estimated depth is computed from weather_5m.snowdistance and stored in inches
-- for consistency with existing snow metric patterns.
--
-- Update pattern:
-- - Periodic job runs every 15 minutes
-- - Processes new data from weather_5m with 6-hour overlap for smoothing boundaries
-- - Initial backfill starts from season start (Oct 1) for new stations
--
-- Algorithm:
-- 1. Convert snowdistance to depth: (base_distance - snowdistance) / 25.4 inches
-- 2. Apply local upper-quantile smoothing (e.g., 85th percentile in 30-min window)
-- 3. Apply rate limiting (max inches/hour up and down)
-- 4. Store result in this table
--
-- Usage:
-- SmoothedComputer queries this table for min/max depth over time periods
-- to calculate 24h, 72h, and seasonal snowfall totals.

-- =============================================================================
-- Create hypertable for estimated snow depth
-- =============================================================================

CREATE TABLE IF NOT EXISTS snow_depth_est_5m (
    time               TIMESTAMPTZ      NOT NULL,
    stationname        TEXT             NOT NULL,
    snow_depth_est_in  DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (stationname, time)
);

-- Convert to TimescaleDB hypertable partitioned by time
SELECT create_hypertable('snow_depth_est_5m', 'time', if_not_exists => TRUE);

-- =============================================================================
-- Create indexes for efficient queries
-- =============================================================================

-- Index for time-range queries (used by SmoothedComputer.Compute24h/72h/Seasonal)
CREATE INDEX IF NOT EXISTS snow_depth_est_5m_stationname_time_idx
ON snow_depth_est_5m (stationname, time DESC);

-- =============================================================================
-- Add documentation comments
-- =============================================================================

COMMENT ON TABLE snow_depth_est_5m IS
'Stores estimated snow depth at 5-minute resolution using smoothed sensor readings.
Updated every 15 minutes by the snowcache controller using quantile smoothing + rate limiting.
Used by SmoothedComputer for snowfall calculations.';

COMMENT ON COLUMN snow_depth_est_5m.snow_depth_est_in IS
'Estimated snow depth in inches, computed from weather_5m.snowdistance using:
1. Local upper-quantile smoothing (removes noise)
2. Rate limiting (enforces realistic accumulation/settling rates)
Result is a physically-plausible depth time series.';
