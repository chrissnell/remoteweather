-- Migration 017: Fix seasonal snowfall to sum all positive deltas
--
-- Problem: Using dual-threshold algorithm with weather_1d produces nonsensical results
-- where seasonal total < 24h total < 72h total. The dual-threshold algorithm was
-- designed for granular data (5m/1h) where it tracks baselines period-by-period.
--
-- With daily aggregates, the 20mm/15mm thresholds prevent small daily accumulations
-- from being counted, and the baseline tracking logic doesn't make sense when data
-- is already smoothed to daily averages.
--
-- Solution: For seasonal calculations on weather_1d, use a simple positive delta sum:
-- - Compare each day to the previous day
-- - If depth increased, add the delta
-- - Daily smoothing has already eliminated false positives from noise
-- - This ensures seasonal >= 72h >= 24h (logical ordering)
--
-- This approach works because:
-- - Daily averaging filters intraday fluctuations (melting, drifting, settling)
-- - We want to capture ALL snowfall days, not just major events
-- - Matches how meteorologists calculate seasonal snowfall

-- =============================================================================
-- Create simple positive delta function for daily aggregates
-- =============================================================================

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
        -- Daily smoothing already eliminated false positives from noise
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
Used for seasonal calculations where daily smoothing has already eliminated noise.
Unlike dual-threshold algorithm, this captures all accumulation days regardless of amount.';

-- =============================================================================
-- Update seasonal function to use simple positive delta sum
-- =============================================================================

DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

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

    -- Use simple positive delta sum for daily aggregates
    -- Daily smoothing already eliminated noise, so just sum all increases
    RETURN get_new_snow_simple_positive_delta(
        p_stationname,
        p_base_distance,
        time_window
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall by summing all positive daily changes from weather_1d.
Ensures seasonal >= 72h >= 24h (logical ordering).
Used by cache refresh job, not called directly by API handlers.';
