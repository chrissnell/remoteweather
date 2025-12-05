-- Migration 020: Use daily data with simple delta for seasonal calculations
--
-- Problem: Hourly dual-threshold accumulates settling/compaction over long periods
-- - 6 days with hourly dual-threshold: 173mm (6.8") - OVERCOUNTING
-- - 6 days with daily simple delta: 30mm (1.2") - REALISTIC
--
-- Root cause: Even dual-threshold on hourly data accumulates noise over months:
-- - Small hourly settling/compaction events (5-10mm)
-- - Wind redistribution causing minor depth changes
-- - Natural snowpack evolution (settling, crusting)
-- - These tiny hourly changes sum to unrealistic totals over seasonal periods
--
-- Solution: Tiered approach based on time window
-- - Short-term (24h, 72h): weather_1h + dual-threshold
--   * Captures real accumulation events within days
--   * Dual-threshold effectively filters sensor noise
--
-- - Long-term (seasonal): weather_1d + simple delta
--   * Daily averaging naturally filters hourly fluctuations
--   * Simple delta captures all real snowfall days
--   * Prevents accumulation of settling/compaction over months
--   * Daily averaging does the same filtering job as dual-threshold

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
    -- Daily averaging filters hourly noise (settling, wind, sensor variance)
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
        -- Daily smoothing already eliminated false positives from hourly noise
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
Daily averaging filters hourly fluctuations (settling, compaction, wind redistribution).
Used for seasonal calculations where daily smoothing prevents accumulation of noise.
Unlike dual-threshold on hourly data, this produces realistic totals over long periods.';

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

    -- Use simple positive delta on daily aggregates
    -- Daily averaging does the filtering, so we don't need dual-threshold
    RETURN get_new_snow_simple_positive_delta(
        p_stationname,
        p_base_distance,
        time_window
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_total_season_snowfall(TEXT, FLOAT) IS
'Calculates total seasonal snowfall using simple positive delta on weather_1d.
Daily averaging prevents accumulation of hourly settling/compaction noise.
Produces realistic totals over long seasonal periods (e.g., 1.2" vs 6.8" overcount).
Used by cache refresh job, not called directly by API handlers.';
