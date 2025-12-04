-- Migration 018 rollback: Restore weather_1d for seasonal calculations
-- (This reverts to using daily data which has limited history and causes ordering issues)

DROP FUNCTION IF EXISTS get_new_snow_simple_positive_delta(TEXT, FLOAT, INTERVAL);

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
Used for seasonal calculations (problematic due to limited historical data).';

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using weather_1d (limited historical data).
Used by cache refresh job, not called directly by API handlers.';
