-- Migration 019: Use dual-threshold algorithm for seasonal calculations
--
-- Problem: Algorithm inconsistency causes ordering issues
-- - 24h/72h: dual-threshold algorithm on weather_1h (captures significant events)
-- - Seasonal: simple positive delta (captures every tiny fluctuation OR misses intraday events)
--
-- Root cause: Mixing algorithms produces incomparable results:
-- - Dual-threshold filters noise and tracks baselines (94mm for 72h)
-- - Simple delta on hourly data: counts every fluctuation (215mm - too high)
-- - Simple delta on daily data: misses intraday accumulation (34mm - too low)
--
-- Solution: Use dual-threshold for ALL calculations including seasonal
-- - Ensures algorithm consistency across all time windows
-- - Captures the same accumulation events that 24h/72h capture
-- - Guarantees logical ordering: seasonal >= 72h >= 24h
-- - Hourly data provides both filtering (via dual-threshold) and completeness

DROP FUNCTION IF EXISTS get_new_snow_simple_positive_delta(TEXT, FLOAT, INTERVAL);

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

    -- Use dual-threshold algorithm on weather_1h for consistency
    -- Same algorithm as 24h/72h ensures comparable, consistent results
    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        time_window,
        'weather_1h'
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using dual-threshold algorithm on weather_1h.
Uses same algorithm as 24h/72h calculations for consistency and logical ordering.
Captures all significant accumulation events while filtering sensor noise.
Used by cache refresh job, not called directly by API handlers.';
