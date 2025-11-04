-- Migration 012: Fix snow season dates to September 1 - June 1
--
-- Problem: Season dates incorrectly set to July 1 / October 1 in various migrations
-- Correct: Snow season should be September 1 through June 1 (next year)

DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
DECLARE
    season_start DATE;
BEGIN
    -- Snow season starts September 1st, ends June 1st (following year)
    -- Sept-Dec: Use current year's Sept 1
    -- Jan-Aug: Use previous year's Sept 1
    IF EXTRACT(MONTH FROM now()) >= 9 THEN
        season_start := DATE_TRUNC('year', now())::DATE + INTERVAL '8 months';
    ELSE
        season_start := DATE_TRUNC('year', now() - INTERVAL '1 year')::DATE + INTERVAL '8 months';
    END IF;

    RETURN get_new_snow_dual_threshold(
        p_stationname,
        p_base_distance,
        now() - season_start::TIMESTAMP
    );
END;
$$ LANGUAGE plpgsql;
