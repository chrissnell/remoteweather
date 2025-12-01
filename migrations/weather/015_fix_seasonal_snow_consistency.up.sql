-- Migration 015: Fix seasonal snow calculation to use weather_5m for consistency
--
-- Problem: Seasonal calculation uses weather_1h while other metrics (24h, 72h, midnight)
-- use weather_5m, causing inconsistent results. When 2.3" of snow falls on the first
-- day of the season, all metrics should show the same value, but they don't because
-- the dual-threshold algorithm behaves differently on different time resolutions.
--
-- Solution: Change seasonal calculation to use weather_5m for consistency with other
-- metrics. Background job performance doesn't matter (500ms is acceptable), only
-- REST endpoint response time matters (which is fast due to caching).

DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
DECLARE
    season_start DATE;
    time_window INTERVAL;
BEGIN
    -- Snow season starts September 1st
    IF EXTRACT(MONTH FROM now()) >= 9 THEN
        season_start := DATE_TRUNC('year', now())::DATE + INTERVAL '8 months';
    ELSE
        season_start := DATE_TRUNC('year', now() - INTERVAL '1 year')::DATE + INTERVAL '8 months';
    END IF;

    time_window := now() - season_start::TIMESTAMP;

    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        time_window,
        'weather_5m'  -- Use 5-minute aggregates for consistency with other metrics
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using weather_5m for consistency with other metrics.
Used by cache refresh job, not called directly by API handlers.';
