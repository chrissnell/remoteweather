-- Migration 022 rollback: Restore SQL-based 72h + seasonal calculations
--
-- This reverts back to the TimescaleDB job + SQL function approach
-- for all snow calculations (midnight, 24h, 72h, seasonal)

-- =============================================================================
-- STEP 1: Restore helper function for simple positive delta (used by seasonal)
-- =============================================================================

CREATE OR REPLACE FUNCTION get_new_snow_simple_positive_delta(
    p_stationname TEXT,
    p_base_distance FLOAT,
    p_time_window INTERVAL
) RETURNS FLOAT AS $$
DECLARE
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
    daily_delta FLOAT;
    rec RECORD;
BEGIN
    -- Query weather_1d and sum all positive day-to-day changes
    -- Daily averaging filters hourly noise (settling, wind, sensor variance)
    FOR rec IN
        SELECT snowdistance
        FROM weather_1d
        WHERE stationname = p_stationname
          AND bucket >= now() - p_time_window
          AND snowdistance IS NOT NULL
          AND snowdistance < p_base_distance - 2
        ORDER BY bucket ASC
    LOOP
        current_distance := rec.snowdistance;
        current_depth := p_base_distance - current_distance;

        -- Skip first reading (establish baseline)
        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        -- Calculate day-to-day change
        daily_delta := current_depth - prev_depth;

        -- Add all positive deltas (snow accumulation)
        IF daily_delta > 0 THEN
            total_accumulation := total_accumulation + daily_delta;
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_simple_positive_delta(TEXT, FLOAT, INTERVAL) IS
'Sums all positive day-to-day snow depth changes from weather_1d table.
Daily averaging filters hourly fluctuations (settling, compaction, wind redistribution).
Used for seasonal and 72h calculations where daily smoothing prevents accumulation of noise.';

-- =============================================================================
-- STEP 2: Restore get_new_snow_72h SQL function (using daily data)
-- =============================================================================

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

-- =============================================================================
-- STEP 3: Restore calculate_total_season_snowfall SQL function
-- =============================================================================

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

    -- Use simple positive delta on daily aggregates
    RETURN get_new_snow_simple_positive_delta(
        p_stationname,
        p_base_distance,
        time_window
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using simple positive delta on weather_1d.
Daily averaging prevents accumulation of hourly settling/compaction noise.
Used by cache refresh job, not called directly by API handlers.';

-- =============================================================================
-- STEP 4: Restore cache refresh function
-- =============================================================================

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
        -- Log warning if called without proper configuration
        RAISE WARNING 'refresh_snow_cache called without stationname or base_distance';
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_snow_cache(INT, JSONB) IS
'Refreshes snow totals cache for a specific station. Called by TimescaleDB job every 30 seconds.
Application must provide stationname and base_distance from device configuration.';

-- =============================================================================
-- STEP 5: Restore TimescaleDB job
-- =============================================================================

DO $$
BEGIN
    -- Check if job already exists before creating
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE proc_name = 'refresh_snow_cache'
    ) THEN
        -- Create job that runs every 30 seconds
        -- The application will need to configure this with the correct parameters
        PERFORM add_job(
            'refresh_snow_cache',
            '30 seconds',
            initial_start => NOW(),
            config => '{"notice": "Job created but needs configuration from application"}'::jsonb
        );
        RAISE NOTICE 'Created TimescaleDB job for refresh_snow_cache';
    ELSE
        RAISE NOTICE 'TimescaleDB job for refresh_snow_cache already exists';
    END IF;
END $$;

-- =============================================================================
-- STEP 6: Restore original table comment
-- =============================================================================

COMMENT ON TABLE snow_totals_cache IS
'Caches pre-computed snow totals to avoid expensive real-time calculations.
Refreshed every 30 seconds by TimescaleDB job. Frontend queries this instead
of calling snow calculation functions directly.';
