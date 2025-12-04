-- Migration 016: Optimize snow table selection for accuracy and performance
--
-- Problem: Using weather_5m for all calculations causes accuracy issues:
-- - Seasonal (3+ months): Too much granularity (~26K rows) captures natural fluctuations
--   (melting, drifting, settling) as snowfall, inflating totals
-- - 24h/72h: Unnecessary granularity when hourly aggregates are sufficient
--
-- Solution: Match aggregation interval to time window for optimal accuracy:
-- - Seasonal: Use weather_1d (~93 rows for 3 months)
--   * Daily totals filter out intraday fluctuations
--   * Matches meteorological reporting standards
--   * 289x fewer rows than weather_5m
-- - 24h/72h: Use weather_1h (24-72 rows)
--   * Hourly smoothing reduces sensor noise
--   * Similar freshness (1h lag vs 5m lag acceptable)
--   * 12x fewer rows than weather_5m
-- - Midnight: Keep weather_5m for maximum freshness (5m lag)
--
-- Performance impact:
-- - Seasonal: Fewer false positives from natural variance
-- - Background job: Slightly slower but still acceptable (<2s vs <1s)
-- - Accuracy: Significantly improved seasonal totals

-- =============================================================================
-- Update 72h function to use weather_1h
-- =============================================================================

DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        interval '72 hours',
        'weather_1h'  -- Hourly aggregates smooth sensor noise
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_72h(TEXT, FLOAT) IS
'Calculates snowfall in last 72 hours using weather_1h (1-hour lag).
Hourly smoothing reduces false positives from sensor variance.
Used by cache refresh job, not called directly by API handlers.';

-- =============================================================================
-- Update 24h function to use weather_1h
-- =============================================================================

DROP FUNCTION IF EXISTS get_new_snow_24h(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        interval '24 hours',
        'weather_1h'  -- Hourly aggregates smooth sensor noise
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_24h(TEXT, FLOAT) IS
'Calculates snowfall in last 24 hours using weather_1h (1-hour lag).
Hourly smoothing reduces false positives from sensor variance.
Used by cache refresh job, not called directly by API handlers.';

-- =============================================================================
-- Update midnight function comment (no change, still uses weather_5m)
-- =============================================================================

COMMENT ON FUNCTION get_new_snow_midnight(TEXT, FLOAT) IS
'Calculates snowfall since midnight using weather_5m (5-minute lag).
Keeps weather_5m for maximum freshness on current day totals.
Used by cache refresh job, not called directly by API handlers.';

-- =============================================================================
-- Update seasonal function to use weather_1d
-- =============================================================================

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
        'weather_1d'  -- Daily aggregates filter intraday fluctuations
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using weather_1d (1-day lag).
Daily totals eliminate false positives from natural variance (melting, drifting).
Matches meteorological reporting standards for snowfall accumulation.
Used by cache refresh job, not called directly by API handlers.';
