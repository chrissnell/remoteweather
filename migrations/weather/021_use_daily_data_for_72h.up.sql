-- Migration 021: Use daily data for 72h calculations
--
-- Rationale: If 6 days of hourly data overcounts (173mm vs 30mm realistic),
-- then 72h (3 days) may also accumulate settling/compaction noise.
-- Using daily averaging for 72h provides more realistic totals.

DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    -- Use simple positive delta on weather_1d for 72h
    -- Daily averaging filters hourly settling/compaction noise
    RETURN get_new_snow_simple_positive_delta(
        p_stationname,
        p_base_distance,
        interval '72 hours'
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_72h(TEXT, FLOAT) IS
'Calculates 72h snowfall using simple positive delta on weather_1d.
Daily averaging prevents accumulation of hourly settling/compaction noise.
Used by cache refresh job, not called directly by API handlers.';
