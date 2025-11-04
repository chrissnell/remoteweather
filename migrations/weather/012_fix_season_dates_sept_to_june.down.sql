-- Rollback: Restore previous (incorrect) July 1st season start

DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
DECLARE
    season_start DATE;
BEGIN
    -- Snow season starts July 1st (INCORRECT - for rollback only)
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
