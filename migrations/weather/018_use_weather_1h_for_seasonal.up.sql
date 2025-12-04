-- Migration 018: Use weather_1h for seasonal calculations instead of weather_1d
--
-- Problem: weather_1d only has recent data (5 days) while weather_1h has historical data.
-- Additionally, using simple positive delta on daily data loses intraday accumulation
-- that the dual-threshold algorithm captures on hourly data, causing:
--   seasonal (1.5") < 72h (3.7") < 24h (1.6") -- impossible!
--
-- Root cause: Mixing algorithms
-- - Seasonal: Simple positive delta on weather_1d (loses intraday detail)
-- - 72h/24h: Dual-threshold on weather_1h (captures intraday cycles)
--
-- Solution: Use weather_1h with simple positive delta for seasonal
-- - Ensures data availability (weather_1h has full history)
-- - Uses same time resolution as 24h/72h for consistency
-- - Simple positive delta guarantees: seasonal >= 72h >= 24h
-- - Captures all hourly accumulations that contribute to seasonal total

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
    hourly_delta FLOAT;
    rec RECORD;
BEGIN
    -- Query weather_1h and sum all positive hour-to-hour changes
    -- Using hourly data ensures: 1) data availability, 2) seasonal >= 72h >= 24h
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
        -- Hourly smoothing already eliminated most sensor noise
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
Used for seasonal calculations where hourly data provides both historical coverage
and ensures seasonal >= 72h >= 24h. Unlike dual-threshold algorithm, this captures
all accumulation hours regardless of amount, guaranteeing logical ordering.';

-- Seasonal function remains the same, but now uses weather_1h data through the updated function
COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall by summing all positive hourly changes from weather_1h.
Uses simple positive delta algorithm to ensure seasonal >= 72h >= 24h (logical ordering).
Used by cache refresh job, not called directly by API handlers.';
