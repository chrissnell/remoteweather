-- Optimize the get_rainfall_with_recent function to avoid cross joins
CREATE OR REPLACE FUNCTION get_rainfall_with_recent(p_stationname TEXT)
RETURNS TABLE(rain_24h REAL, rain_48h REAL, rain_72h REAL) AS $$
DECLARE
    v_rain_24h REAL;
    v_rain_48h REAL;
    v_rain_72h REAL;
    v_last_updated TIMESTAMPTZ;
    v_recent_rain REAL;
BEGIN
    -- Get the summary data
    SELECT rs.rain_24h, rs.rain_48h, rs.rain_72h, rs.last_updated
    INTO v_rain_24h, v_rain_48h, v_rain_72h, v_last_updated
    FROM rainfall_summary rs
    WHERE rs.stationname = p_stationname
    LIMIT 1;
    
    -- If no summary exists, calculate from scratch
    IF NOT FOUND THEN
        RETURN QUERY
        SELECT 
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '24 hours' THEN period_rain END), 0)::REAL,
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '48 hours' THEN period_rain END), 0)::REAL,
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '72 hours' THEN period_rain END), 0)::REAL
        FROM weather_5m
        WHERE stationname = p_stationname 
        AND bucket >= NOW() - INTERVAL '72 hours';
        RETURN;
    END IF;
    
    -- Get recent rain since last update
    SELECT COALESCE(SUM(rainincremental), 0)
    INTO v_recent_rain
    FROM weather
    WHERE stationname = p_stationname
    AND time > v_last_updated;
    
    -- Return combined values
    RETURN QUERY 
    SELECT 
        (v_rain_24h + v_recent_rain)::REAL,
        (v_rain_48h + v_recent_rain)::REAL,
        (v_rain_72h + v_recent_rain)::REAL;
END;
$$ LANGUAGE plpgsql;

-- Replace the today_rainfall view with a more efficient version
DROP VIEW IF EXISTS today_rainfall;

CREATE VIEW today_rainfall AS
WITH recent_weather AS (
    SELECT COALESCE(SUM(rainincremental), 0) as recent_rain
    FROM weather
    WHERE time >= GREATEST(
        date_trunc('day', now()),
        (SELECT COALESCE(MAX(bucket), date_trunc('day', now())) FROM weather_5m)
    )
)
SELECT
    COALESCE(
        (SELECT SUM(period_rain) FROM weather_5m WHERE bucket >= date_trunc('day', now())),
        0
    ) + COALESCE((SELECT recent_rain FROM recent_weather), 0) AS total_rain;

-- Add index to speed up rainfall_summary lookups
CREATE INDEX IF NOT EXISTS rainfall_summary_stationname_idx ON rainfall_summary (stationname);

-- Ensure weather table has proper index for time-based queries
CREATE INDEX IF NOT EXISTS weather_time_stationname_idx ON weather (time DESC, stationname);