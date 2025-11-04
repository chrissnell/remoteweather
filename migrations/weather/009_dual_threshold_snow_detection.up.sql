-- Implement dual-threshold snow detection algorithm
--
-- Previous approach: Single 10mm threshold caused false positives from sensor noise
-- New approach: Dual thresholds to handle both rapid accumulation and gradual buildup
--
-- Quick threshold (20mm): Detects storms with rapid accumulation
-- Gradual threshold (15mm): Catches light steady snow that accumulates over time
-- Baseline tracking: Only counts increases, ignores normal fluctuations

-- Core dual-threshold function with configurable time window
CREATE OR REPLACE FUNCTION get_new_snow_dual_threshold(
    p_stationname TEXT,
    p_base_distance FLOAT,
    p_time_window INTERVAL
) RETURNS FLOAT AS $$
DECLARE
    quick_threshold FLOAT := 20.0;      -- 0.8" rapid accumulation in one hour
    gradual_threshold FLOAT := 15.0;    -- 0.6" gradual accumulation from baseline
    melt_threshold FLOAT := 10.0;       -- >10mm drop indicates melting/sublimation

    total_accumulation FLOAT := 0.0;
    baseline_depth FLOAT := NULL;       -- High-water mark for gradual accumulation
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
    hourly_delta FLOAT;
BEGIN
    -- Iterate through hourly readings in chronological order
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - p_time_window
          AND snowdistance IS NOT NULL
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

        -- First reading - establish initial baselines
        IF prev_depth IS NULL THEN
            baseline_depth := current_depth;
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        -- Calculate change from previous hour
        hourly_delta := current_depth - prev_depth;

        -- MODE 1: Quick accumulation detection (storm/heavy snow)
        -- If depth increased by >20mm in one hour, count it immediately
        IF hourly_delta > quick_threshold THEN
            total_accumulation := total_accumulation + hourly_delta;
            baseline_depth := current_depth;  -- Reset baseline after counting

        -- MODE 2: Gradual accumulation detection (light steady snow)
        -- If current depth exceeds baseline by >15mm, count the increase
        -- This catches small hourly increases that add up over time
        ELSIF current_depth > baseline_depth + gradual_threshold THEN
            total_accumulation := total_accumulation + (current_depth - baseline_depth);
            baseline_depth := current_depth;  -- Update baseline to new level

        -- RESET: Significant depth decrease (melting/sublimation/blowing)
        -- Reset baseline to current level to avoid counting old snow
        ELSIF current_depth < baseline_depth - melt_threshold THEN
            baseline_depth := current_depth;
        END IF;

        -- Update previous depth for next iteration
        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- Replace get_new_snow_24h with dual-threshold version
CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    RETURN QUERY SELECT get_new_snow_dual_threshold(p_stationname, p_base_distance, '24 hours'::interval);
END;
$$ LANGUAGE plpgsql;

-- Replace get_new_snow_72h with dual-threshold version
CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    RETURN QUERY SELECT get_new_snow_dual_threshold(p_stationname, p_base_distance, '72 hours'::interval);
END;
$$ LANGUAGE plpgsql;

-- Replace get_new_snow_midnight with dual-threshold version
CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    -- Calculate interval from midnight to now
    RETURN QUERY SELECT get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        (now() - date_trunc('day', now()))
    );
END;
$$ LANGUAGE plpgsql;

-- Replace calculate_total_season_snowfall with dual-threshold version
CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
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

    local_start_of_season TIMESTAMPTZ;
    season_end TIMESTAMPTZ;
    current_year INTEGER;
    current_month INTEGER;
BEGIN
    -- Determine the current snow season (October 1 to May 1)
    IF start_of_season IS NULL THEN
        current_year := extract(YEAR FROM now())::INT;
        current_month := extract(MONTH FROM now())::INT;

        IF current_month >= 10 THEN
            -- October-December: current season
            local_start_of_season := make_timestamptz(current_year, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSIF current_month <= 4 THEN
            -- January-April: season started previous year
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSE
            -- May-September: off-season, use most recent completed season
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        END IF;
    ELSE
        local_start_of_season := start_of_season;
    END IF;

    season_end := local_start_of_season + interval '7 months';

    -- Iterate through hourly readings for the season
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= local_start_of_season
          AND bucket < season_end
          AND snowdistance IS NOT NULL
        ORDER BY bucket ASC
    LOOP
        current_depth := base_distance - current_distance;

        IF prev_depth IS NULL THEN
            baseline_depth := current_depth;
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        hourly_delta := current_depth - prev_depth;

        -- Quick accumulation
        IF hourly_delta > quick_threshold THEN
            total_accumulation := total_accumulation + hourly_delta;
            baseline_depth := current_depth;

        -- Gradual accumulation
        ELSIF current_depth > baseline_depth + gradual_threshold THEN
            total_accumulation := total_accumulation + (current_depth - baseline_depth);
            baseline_depth := current_depth;

        -- Reset on melt
        ELSIF current_depth < baseline_depth - melt_threshold THEN
            baseline_depth := current_depth;
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- Update storm snowfall calculation to use dual-threshold approach
CREATE OR REPLACE FUNCTION calculate_storm_snowfall(
    p_stationname TEXT
) RETURNS TABLE(
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_snowfall FLOAT
) AS $$
DECLARE
    storm_start_ts TIMESTAMPTZ;
    storm_end_ts TIMESTAMPTZ;
    total_snowfall_amount FLOAT;
    base_dist FLOAT;
BEGIN
    -- Get base distance for this station from device config
    -- Note: This is a simplified version - actual base_distance should come from application
    -- For now, use a reasonable default or the function should receive it as parameter
    base_dist := 1798.0;  -- TODO: Pass this as parameter from application

    -- Calculate total snowfall in last 24 hours using dual-threshold method
    SELECT get_new_snow_dual_threshold(p_stationname, base_dist, '24 hours'::interval)
    INTO total_snowfall_amount;

    -- If no significant snowfall, return no storm
    IF total_snowfall_amount IS NULL OR total_snowfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Set storm period as last 24 hours
    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;
