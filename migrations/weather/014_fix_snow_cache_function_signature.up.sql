-- Migration 014: Fix refresh_snow_cache function signature for TimescaleDB job compatibility
--
-- Problem: TimescaleDB jobs pass (job_id, config) as first two parameters, but migration 013
-- created the function with them as the last two parameters, causing "cache lookup failed" errors.
--
-- Solution: Drop old function and recreate with correct parameter order.

-- Drop the incorrectly-ordered function
DROP FUNCTION IF EXISTS refresh_snow_cache(TEXT, FLOAT, INT, JSONB);

-- Recreate with correct parameter order (job_id and config MUST be first two parameters)
CREATE OR REPLACE FUNCTION refresh_snow_cache(
    job_id INT DEFAULT NULL,
    config JSONB DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    station_name TEXT;
    base_distance FLOAT;
BEGIN
    -- Extract parameters from config (always provided by TimescaleDB job)
    IF config IS NOT NULL THEN
        station_name := config->>'stationname';
        base_distance := (config->>'base_distance')::FLOAT;
    ELSE
        -- Should never happen when called by TimescaleDB job
        RAISE EXCEPTION 'refresh_snow_cache requires config parameter';
    END IF;

    -- Refresh cache if we have both parameters
    IF station_name IS NOT NULL AND base_distance IS NOT NULL THEN
        INSERT INTO snow_totals_cache (
            stationname,
            snow_midnight,
            snow_24h,
            snow_72h,
            snow_season,
            base_distance,
            computed_at
        ) VALUES (
            station_name,
            get_new_snow_midnight(station_name, base_distance),
            get_new_snow_24h(station_name, base_distance),
            get_new_snow_72h(station_name, base_distance),
            calculate_total_season_snowfall(station_name, base_distance),
            base_distance,
            now()
        )
        ON CONFLICT (stationname) DO UPDATE SET
            snow_midnight = EXCLUDED.snow_midnight,
            snow_24h = EXCLUDED.snow_24h,
            snow_72h = EXCLUDED.snow_72h,
            snow_season = EXCLUDED.snow_season,
            base_distance = EXCLUDED.base_distance,
            computed_at = EXCLUDED.computed_at;
    ELSE
        RAISE WARNING 'refresh_snow_cache called without stationname or base_distance';
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_snow_cache(INT, JSONB) IS
'Refreshes snow totals cache for a specific station. Called by TimescaleDB job every 30 seconds.
TimescaleDB jobs pass (job_id, config) as first two parameters.
Application configures job with stationname and base_distance from device configuration.';
