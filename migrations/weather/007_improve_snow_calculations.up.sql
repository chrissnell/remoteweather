-- Migration 007: Improve snow calculation functions to eliminate sensor noise
--
-- Problem: Ultrasonic snow sensor has ±1% variability (~10-20mm at 1800mm distance)
-- This creates phantom snow accumulation when there's no real snow
--
-- Solution:
-- 1. Use hourly averages (weather_1h) instead of raw/daily data - smooths noise
-- 2. Add 10mm detection threshold - ignores sensor variation
-- 3. Only count depth increases > threshold as real snow
--
-- Testing on 2025-10-01 to 2025-11-02 (zero real snow):
-- - Old algorithm: 104mm phantom snow
-- - New algorithm: 0mm phantom snow

-- Create configuration function for detection threshold
CREATE OR REPLACE FUNCTION get_snow_detection_threshold() RETURNS FLOAT AS $$
BEGIN
    RETURN 10.0;  -- 10mm threshold eliminates sensor noise (tested on real data)
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- ============================================================================
-- FUNCTION: get_new_snow_72h
-- Calculates snowfall in the last 72 hours using hourly averages
-- ============================================================================
CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := get_snow_detection_threshold();
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := 0.0;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    -- Iterate through hourly averages (smooths sensor noise)
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - interval '72 hours'
          AND snowdistance IS NOT NULL
          AND snowdistance <= p_base_distance
        ORDER BY bucket ASC
    LOOP
        -- Calculate snow depth (distance from sensor decreased = more snow)
        current_depth := p_base_distance - current_distance;

        -- Only add to total if depth increased by more than threshold
        IF current_depth > prev_depth + threshold THEN
            total_accumulation := total_accumulation + (current_depth - prev_depth);
        END IF;

        -- Always update prev_depth to handle melting correctly
        prev_depth := current_depth;
    END LOOP;

    RETURN QUERY SELECT total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: get_new_snow_24h
-- Calculates snowfall in the last 24 hours using hourly averages
-- ============================================================================
CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := get_snow_detection_threshold();
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := 0.0;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    -- Iterate through hourly averages (smooths sensor noise)
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - interval '24 hours'
          AND snowdistance IS NOT NULL
          AND snowdistance <= p_base_distance
        ORDER BY bucket ASC
    LOOP
        -- Calculate snow depth
        current_depth := p_base_distance - current_distance;

        -- Only add to total if depth increased by more than threshold
        IF current_depth > prev_depth + threshold THEN
            total_accumulation := total_accumulation + (current_depth - prev_depth);
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN QUERY SELECT total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: get_new_snow_midnight
-- Calculates snowfall since midnight using hourly averages
-- ============================================================================
CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := get_snow_detection_threshold();
    midnight TIMESTAMPTZ;
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := 0.0;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    midnight := date_trunc('day', now());

    -- Iterate through hourly averages since midnight
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= midnight
          AND snowdistance IS NOT NULL
          AND snowdistance <= p_base_distance
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

        IF current_depth > prev_depth + threshold THEN
            total_accumulation := total_accumulation + (current_depth - prev_depth);
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN QUERY SELECT total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: calculate_total_season_snowfall
-- Calculates total snowfall for the season using hourly averages
-- Season: October 1 - May 1
-- ============================================================================
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    threshold FLOAT := get_snow_detection_threshold();
    total_snowfall FLOAT := 0.0;
    prev_depth FLOAT := 0.0;
    current_depth FLOAT;
    current_distance FLOAT;
    local_start_of_season TIMESTAMPTZ;
    season_end TIMESTAMPTZ;
    current_year INTEGER;
    current_month INTEGER;
BEGIN
    -- Determine the current snow season (October 1 to May 1)
    IF start_of_season IS NULL THEN
        current_year := extract(YEAR FROM now())::INT;
        current_month := extract(MONTH FROM now())::INT;

        -- Determine which season we're in
        IF current_month >= 10 THEN
            -- October-December: current season (Oct 1 current year to May 1 next year)
            local_start_of_season := make_timestamptz(current_year, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSIF current_month <= 4 THEN
            -- January-April: current season (Oct 1 previous year to May 1 current year)
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSE
            -- May-September: off-season, use most recent completed season
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        END IF;
    ELSE
        local_start_of_season := start_of_season;
    END IF;

    -- Calculate season end (May 1 of the following year)
    season_end := local_start_of_season + interval '7 months';

    -- Iterate through HOURLY averages (much smoother than daily)
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= local_start_of_season
          AND bucket < season_end
          AND snowdistance IS NOT NULL
          AND snowdistance <= base_distance
        ORDER BY bucket ASC
    LOOP
        -- Calculate snow depth
        current_depth := base_distance - current_distance;

        -- Only add to season total if depth increased by more than threshold
        IF current_depth > prev_depth + threshold THEN
            total_snowfall := total_snowfall + (current_depth - prev_depth);
        END IF;

        -- Always update prev_depth to track current state
        prev_depth := current_depth;
    END LOOP;

    -- Return the total snowfall, ensuring it's not negative
    RETURN GREATEST(total_snowfall, 0.0);
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: calculate_storm_snowfall
-- Calculates snowfall during a storm (defined as snow in last 24 hours)
-- Now uses the improved 24h function with threshold logic
-- ============================================================================
CREATE OR REPLACE FUNCTION calculate_storm_snowfall(
    p_stationname TEXT
) RETURNS TABLE (
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_snowfall FLOAT
) AS $$
DECLARE
    storm_start_ts TIMESTAMPTZ;
    storm_end_ts TIMESTAMPTZ;
    total_snowfall_amount FLOAT := 0.0;
    -- This is a temporary hardcoded value - should match your actual base_distance
    -- TODO: Consider making base_distance a configuration table value
    base_dist FLOAT := 1798.0;
BEGIN
    -- Use the improved 24h function which already has threshold logic
    SELECT snowfall INTO total_snowfall_amount
    FROM get_new_snow_24h(p_stationname, base_dist);

    -- If no significant snowfall, return no storm
    IF total_snowfall_amount IS NULL OR total_snowfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Set storm period as last 24 hours
    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    -- Return the storm information
    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;
