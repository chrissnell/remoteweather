-- Rollback migration 007: Restore original snow calculation functions
-- This reverts to the old algorithm that uses raw/daily data without threshold filtering

-- Restore original get_new_snow_72h function
CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;
    latest_reading FLOAT;
BEGIN
    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '72 hours'
     ORDER BY time ASC
     LIMIT 1;

    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '72 hours'
     ORDER BY time DESC
     LIMIT 1;

    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;

-- Restore original get_new_snow_24h function
CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;
    latest_reading FLOAT;
BEGIN
    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '24 hours'
     ORDER BY time ASC
     LIMIT 1;

    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '24 hours'
     ORDER BY time DESC
     LIMIT 1;

    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;

-- Restore original get_new_snow_midnight function
CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;
    latest_reading FLOAT;
    midnight TIMESTAMPTZ;
BEGIN
    midnight := date_trunc('day', now());

    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= midnight
     ORDER BY time ASC
     LIMIT 1;

    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= midnight
     ORDER BY time DESC
     LIMIT 1;

    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;

-- Restore original calculate_total_season_snowfall function
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    total_snowfall FLOAT := 0.0;
    previous_snowdistance FLOAT := NULL;
    current_snowdistance FLOAT;
    current_bucket TIMESTAMPTZ;
    local_start_of_season TIMESTAMPTZ;
    season_end TIMESTAMPTZ;
    current_year INTEGER;
    current_month INTEGER;
    today_snowfall FLOAT := 0.0;
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

    FOR current_bucket, current_snowdistance IN
        SELECT bucket, snowdistance
        FROM weather_1d
        WHERE weather_1d.stationname = p_stationname
          AND bucket >= local_start_of_season
          AND bucket < season_end
          AND snowdistance IS NOT NULL
        ORDER BY bucket
    LOOP
        IF current_snowdistance <= base_distance THEN
            IF previous_snowdistance IS NOT NULL THEN
                IF current_snowdistance < previous_snowdistance THEN
                    total_snowfall := total_snowfall + (previous_snowdistance - current_snowdistance);
                END IF;
            END IF;
            previous_snowdistance := current_snowdistance;
        END IF;
    END LOOP;

    IF now() >= local_start_of_season AND now() < season_end THEN
        DECLARE
            latest_raw_snowdistance FLOAT;
            latest_daily_snowdistance FLOAT;
        BEGIN
            SELECT snowdistance INTO latest_raw_snowdistance
            FROM weather
            WHERE weather.stationname = p_stationname
              AND snowdistance IS NOT NULL
              AND snowdistance <= base_distance
            ORDER BY time DESC
            LIMIT 1;

            SELECT snowdistance INTO latest_daily_snowdistance
            FROM weather_1d
            WHERE weather_1d.stationname = p_stationname
              AND bucket < date_trunc('day', now())
              AND snowdistance IS NOT NULL
            ORDER BY bucket DESC
            LIMIT 1;

            IF latest_raw_snowdistance IS NOT NULL AND latest_daily_snowdistance IS NOT NULL THEN
                today_snowfall := latest_daily_snowdistance - latest_raw_snowdistance;
                IF today_snowfall > 0 THEN
                    total_snowfall := total_snowfall + today_snowfall;
                END IF;
            END IF;
        END;
    END IF;

    RETURN GREATEST(total_snowfall, 0.0);
END;
$$ LANGUAGE plpgsql;

-- Restore original calculate_storm_snowfall function
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
    first_reading FLOAT;
    latest_reading FLOAT;
BEGIN
    SELECT snowdistance INTO first_reading
    FROM weather
    WHERE weather.stationname = p_stationname
      AND time >= now() - interval '24 hours'
      AND snowdistance IS NOT NULL
    ORDER BY time ASC
    LIMIT 1;

    SELECT snowdistance INTO latest_reading
    FROM weather
    WHERE weather.stationname = p_stationname
      AND time >= now() - interval '24 hours'
      AND snowdistance IS NOT NULL
    ORDER BY time DESC
    LIMIT 1;

    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    total_snowfall_amount := first_reading - latest_reading;

    IF total_snowfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;

-- Drop the threshold configuration function (new in migration 007)
DROP FUNCTION IF EXISTS get_snow_detection_threshold();
