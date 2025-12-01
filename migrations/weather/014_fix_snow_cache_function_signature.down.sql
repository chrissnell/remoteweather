-- Migration 014 rollback: Restore buggy function signature
-- (This should never be needed, but included for completeness)

DROP FUNCTION IF EXISTS refresh_snow_cache(INT, JSONB);

-- Restore the incorrect parameter order from migration 013
CREATE OR REPLACE FUNCTION refresh_snow_cache(
    p_stationname TEXT DEFAULT NULL,
    p_base_distance FLOAT DEFAULT NULL,
    job_id INT DEFAULT NULL,
    config JSONB DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    station_name TEXT;
    base_distance FLOAT;
BEGIN
    -- Extract parameters from config if called by TimescaleDB job
    IF p_stationname IS NULL AND config IS NOT NULL THEN
        station_name := config->>'stationname';
        base_distance := (config->>'base_distance')::FLOAT;
    ELSE
        station_name := p_stationname;
        base_distance := p_base_distance;
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
