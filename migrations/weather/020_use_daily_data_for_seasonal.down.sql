-- Migration 020 rollback: Restore dual-threshold on hourly data for seasonal
-- (This reverts to overcounting settling/compaction over long periods)

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

    -- Use dual-threshold on hourly data (overcounts over long periods)
    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        time_window,
        'weather_1h'
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using dual-threshold on weather_1h (overcounts).
Used by cache refresh job, not called directly by API handlers.';
