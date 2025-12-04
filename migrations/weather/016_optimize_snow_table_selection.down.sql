-- Migration 016 rollback: Restore weather_5m for all calculations
-- (Reverts to the problematic state that caused inaccurate seasonal totals)

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
        'weather_5m'  -- Use 5-minute aggregates for consistency
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_72h(TEXT, FLOAT) IS
'Calculates snowfall in last 72 hours using weather_5m (5-minute lag).
Used by cache refresh job, not called directly by API handlers.';

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
        'weather_5m'  -- Use 5-minute aggregates for consistency
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_24h(TEXT, FLOAT) IS
'Calculates snowfall in last 24 hours using weather_5m (5-minute lag).
Used by cache refresh job, not called directly by API handlers.';

COMMENT ON FUNCTION get_new_snow_midnight(TEXT, FLOAT) IS
'Calculates snowfall since midnight using weather_5m (5-minute lag).
Used by cache refresh job, not called directly by API handlers.';

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
        'weather_5m'  -- Use 5-minute aggregates for consistency
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using weather_5m for consistency with other metrics.
Used by cache refresh job, not called directly by API handlers.';
