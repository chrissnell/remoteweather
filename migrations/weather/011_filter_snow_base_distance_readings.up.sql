-- Migration 011: Filter out readings within 2mm of base_distance to prevent false baseline resets
--
-- Problem: When snowdistance equals base_distance (depth=0mm, valid "no snow" reading),
-- the baseline gets reset to 0mm. Later depth increases are then incorrectly counted as new snow.
--
-- Solution: Exclude readings where snowdistance is within 2mm of base_distance from baseline calculations.

DROP FUNCTION IF EXISTS get_new_snow_dual_threshold(TEXT, FLOAT, INTERVAL);

CREATE OR REPLACE FUNCTION get_new_snow_dual_threshold(
    p_stationname TEXT,
    p_base_distance FLOAT,
    p_time_window INTERVAL
) RETURNS FLOAT AS $$
DECLARE
    quick_threshold FLOAT := 20.0;      -- 0.8" rapid accumulation
    gradual_threshold FLOAT := 15.0;    -- 0.6" gradual accumulation
    melt_threshold FLOAT := 10.0;       -- >10mm drop = melting

    total_accumulation FLOAT := 0.0;
    baseline_depth FLOAT := NULL;
    prev_depth FLOAT := NULL;
    current_depth FLOAT;
    current_distance FLOAT;
    hourly_delta FLOAT;
BEGIN
    FOR current_distance IN
        SELECT snowdistance
        FROM weather_1h
        WHERE stationname = p_stationname
          AND bucket >= now() - p_time_window
          AND snowdistance IS NOT NULL
          AND snowdistance < p_base_distance - 2  -- Filter readings within 2mm of base (no/minimal snow)
        ORDER BY bucket ASC
    LOOP
        current_depth := p_base_distance - current_distance;

        IF prev_depth IS NULL THEN
            baseline_depth := current_depth;
            prev_depth := current_depth;
            CONTINUE;
        END IF;

        hourly_delta := current_depth - prev_depth;

        -- Quick accumulation: >20mm in one hour
        IF hourly_delta > quick_threshold THEN
            total_accumulation := total_accumulation + hourly_delta;
            baseline_depth := current_depth;

        -- Gradual accumulation: >15mm above baseline
        ELSIF current_depth > baseline_depth + gradual_threshold THEN
            total_accumulation := total_accumulation + (current_depth - baseline_depth);
            baseline_depth := current_depth;

        -- Reset on significant melt
        ELSIF current_depth < baseline_depth - melt_threshold THEN
            baseline_depth := current_depth;
        END IF;

        prev_depth := current_depth;
    END LOOP;

    RETURN total_accumulation;
END;
$$ LANGUAGE plpgsql;

-- Recreate wrapper functions with DROP IF EXISTS to force replacement
DROP FUNCTION IF EXISTS get_new_snow_24h(TEXT, FLOAT);
DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);
DROP FUNCTION IF EXISTS get_new_snow_midnight(TEXT, FLOAT);
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(p_stationname, p_base_distance, interval '24 hours');
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(p_stationname, p_base_distance, interval '72 hours');
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        now() - date_trunc('day', now() AT TIME ZONE 'America/Denver') AT TIME ZONE 'America/Denver'
    );
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
DECLARE
    season_start DATE;
BEGIN
    -- Snow season starts July 1st
    IF EXTRACT(MONTH FROM now()) >= 7 THEN
        season_start := DATE_TRUNC('year', now())::DATE + INTERVAL '6 months';
    ELSE
        season_start := DATE_TRUNC('year', now() - INTERVAL '1 year')::DATE + INTERVAL '6 months';
    END IF;

    RETURN get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        now() - season_start::TIMESTAMP
    );
END;
$$ LANGUAGE plpgsql;
