-- Revert to the previous (buggy) single-threshold snow detection

-- Remove dual-threshold core function
DROP FUNCTION IF EXISTS get_new_snow_dual_threshold(TEXT, FLOAT, INTERVAL);

-- Revert to migration 008 version (fixed baseline but single threshold)
CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - interval '24 hours'
          AND snowdistance IS NOT NULL
          AND snowdistance <= p_base_distance
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

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

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - interval '72 hours'
          AND snowdistance IS NOT NULL
          AND snowdistance <= p_base_distance
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

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

CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    threshold FLOAT := 10.0;
    midnight TIMESTAMPTZ;
    total_accumulation FLOAT := 0.0;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
BEGIN
    midnight := date_trunc('day', now());

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

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    threshold FLOAT := 10.0;
    total_snowfall FLOAT := 0.0;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
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
          AND snowdistance <= base_distance
        ORDER BY bucket ASC
    LOOP
        current_depth := base_distance - current_distance;

        IF prev_depth IS NULL THEN
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        IF current_depth > prev_depth + threshold THEN
            total_snowfall := total_snowfall + (current_depth - prev_depth);
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_snowfall;
END;
$$ LANGUAGE plpgsql;

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
    base_dist := 1798.0;

    SELECT snowfall FROM get_new_snow_24h(p_stationname, base_dist)
    INTO total_snowfall_amount;

    IF total_snowfall_amount IS NULL OR total_snowfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;
