-- Migration 013 rollback: Remove snow cache and restore original functions

-- Drop the cache refresh job
DO $$
DECLARE
    job_rec RECORD;
BEGIN
    FOR job_rec IN
        SELECT job_id FROM timescaledb_information.jobs
        WHERE proc_name = 'refresh_snow_cache'
    LOOP
        PERFORM delete_job(job_rec.job_id);
    END LOOP;
END $$;

-- Drop cache table and indexes
DROP TABLE IF EXISTS snow_totals_cache CASCADE;

-- Drop the new flexible function
DROP FUNCTION IF EXISTS get_new_snow_dual_threshold_from_table(TEXT, FLOAT, INTERVAL, TEXT);
DROP FUNCTION IF EXISTS refresh_snow_cache(TEXT, FLOAT, INT, JSONB);

-- Restore original functions using weather_1h
-- (These are from migration 011)

CREATE OR REPLACE FUNCTION get_new_snow_dual_threshold(
    p_stationname TEXT,
    p_base_distance FLOAT,
    p_time_window INTERVAL
) RETURNS FLOAT AS $$
DECLARE
    quick_threshold FLOAT := 20.0;
    gradual_threshold FLOAT := 15.0;
    melt_threshold FLOAT := 10.0;

    total_accumulation FLOAT := 0.0;
    baseline_depth FLOAT := NULL;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
    hourly_delta FLOAT;
BEGIN
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - p_time_window
          AND snowdistance IS NOT NULL
          AND snowdistance < p_base_distance - 2
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

        IF prev_depth IS NULL THEN
            baseline_depth := current_depth;
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        hourly_delta := current_depth - prev_depth;

        IF hourly_delta > quick_threshold THEN
            total_accumulation := total_accumulation + hourly_delta;
            baseline_depth := current_depth;

        ELSIF current_depth > baseline_depth + gradual_threshold THEN
            total_accumulation := total_accumulation + (current_depth - baseline_depth);
            baseline_depth := current_depth;

        ELSIF current_depth < baseline_depth - melt_threshold THEN
            baseline_depth := current_depth;
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(p_stationname, p_base_distance, interval '24 hours');
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(p_stationname, p_base_distance, interval '72 hours');
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        now() - date_trunc('day', now() AT TIME ZONE 'America/Denver') AT TIME ZONE 'America/Denver'
    );
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
DECLARE
    season_start DATE;
BEGIN
    IF EXTRACT(MONTH FROM now()) >= 9 THEN
        season_start := DATE_TRUNC('year', now())::DATE + INTERVAL '8 months';
    ELSE
        season_start := DATE_TRUNC('year', now() - INTERVAL '1 year')::DATE + INTERVAL '8 months';
    END IF;

    RETURN get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        now() - season_start::TIMESTAMP
    );
END;
$$ LANGUAGE plpgsql;
