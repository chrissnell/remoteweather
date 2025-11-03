-- Fix prev_depth initialization bug in all snow accumulation functions
--
-- Bug: Functions were initializing prev_depth to 0.0, causing the first reading
-- in the time window to be compared against 0 instead of being used as the baseline.
-- This caused existing snow depth to be incorrectly counted as new snowfall.
--
-- Fix: Initialize prev_depth to NULL and skip the first reading (use it as baseline)

-- Fix get_new_snow_24h
CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;  -- 10mm threshold to filter sensor noise
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;  -- Changed from 0.0 to NULL
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

        -- Skip first reading - just set baseline
        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        -- Only add to total if depth increased by more than threshold
        IF current_depth > prev_depth + threshold THEN
            total_accumulation := total_accumulation + (current_depth - prev_depth);
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN QUERY SELECT total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- Fix get_new_snow_72h
CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;  -- 10mm threshold to filter sensor noise
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;  -- Changed from 0.0 to NULL
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

        -- Skip first reading - just set baseline
        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

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

-- Fix get_new_snow_midnight
CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;  -- 10mm threshold to filter sensor noise
    midnight TIMESTAMPTZ;
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;  -- Changed from 0.0 to NULL
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

        -- Skip first reading - just set baseline
        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        IF current_depth > prev_depth + threshold THEN
            total_accumulation := total_accumulation + (current_depth - prev_depth);
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN QUERY SELECT total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- Fix calculate_total_season_snowfall
CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    threshold FLOAT := 10.0;  -- 10mm threshold to filter sensor noise
    total_snowfall FLOAT := 0.0;
    prev_depth FLOAT := NULL;  -- Changed from 0.0 to NULL
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
        current_depth := base_distance - current_distance;

        -- Skip first reading - just set baseline
        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        -- Only add to total if depth increased by more than threshold
        IF current_depth > prev_depth + threshold THEN
            total_snowfall := total_snowfall + (current_depth - prev_depth);
        END IF;

        -- Always update prev_depth to handle melting/settling
        prev_depth := current_depth;
    END LOOP;

    RETURN total_snowfall;
END;
$$ LANGUAGE plpgsql;
