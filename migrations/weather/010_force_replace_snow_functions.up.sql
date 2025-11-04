-- Force replace snow functions by explicitly dropping old versions first
-- Migration 009 didn't properly replace the functions due to overloading

-- Drop old versions
DROP FUNCTION IF EXISTS get_new_snow_24h(TEXT, FLOAT);
DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);
DROP FUNCTION IF EXISTS get_new_snow_midnight(TEXT, FLOAT);
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

-- Recreate with dual-threshold implementation
CREATE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    RETURN QUERY SELECT get_new_snow_dual_threshold(p_stationname, p_base_distance, '24 hours'::interval);
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    RETURN QUERY SELECT get_new_snow_dual_threshold(p_stationname, p_base_distance, '72 hours'::interval);
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
BEGIN
    RETURN QUERY SELECT get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        (now() - date_trunc('day', now()))
    );
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION calculate_total_season_snowfall(
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
    IF start_of_season IS NULL THEN
        current_year := extract(YEAR FROM now())::INT;
        current_month := extract(MONTH FROM now())::INT;

        IF current_month >= 10 THEN
            local_start_of_season := make_timestamptz(current_year, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSIF current_month <= 4 THEN
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSE
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        END IF;
    ELSE
        local_start_of_season := start_of_season;
    END IF;

    season_end := local_start_of_season + interval '7 months';

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
