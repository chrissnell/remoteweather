-- Migration 019 rollback: Restore simple positive delta for seasonal
-- (This reverts to the algorithm mismatch state)

DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

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
    hourly_delta FLOAT;
    rec RECORD;
BEGIN
    -- Query weather_1h and sum all positive hour-to-hour changes
    FOR rec IN
        SELECT snowdistance
        FROM weather_1h
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

        -- Calculate hour-to-hour change
        hourly_delta := current_depth - prev_depth;

        -- Add all positive deltas (snow accumulation)
        IF hourly_delta > 0 THEN
            total_accumulation := total_accumulation + hourly_delta;
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_simple_positive_delta(TEXT, FLOAT, INTERVAL) IS
'Sums all positive hour-to-hour snow depth changes from weather_1h table.
Used for seasonal calculations (algorithm mismatch with 24h/72h).';

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

    RETURN get_new_snow_simple_positive_delta(
        p_stationname,
        p_base_distance,
        time_window
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using simple positive delta (algorithm mismatch).
Used by cache refresh job, not called directly by API handlers.';
