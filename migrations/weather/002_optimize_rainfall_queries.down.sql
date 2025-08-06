-- Restore original get_rainfall_with_recent function
CREATE OR REPLACE FUNCTION get_rainfall_with_recent(p_stationname TEXT)
RETURNS TABLE(rain_24h REAL, rain_48h REAL, rain_72h REAL) AS $$
BEGIN
    RETURN QUERY
    WITH summary AS (
        SELECT 
            rs.rain_24h, 
            rs.rain_48h, 
            rs.rain_72h, 
            rs.last_updated
        FROM rainfall_summary rs
        WHERE rs.stationname = p_stationname
    ),
    recent AS (
        SELECT COALESCE(SUM(rainincremental), 0) as recent_rain
        FROM weather, summary
        WHERE stationname = p_stationname
        AND time > summary.last_updated
    )
    SELECT 
        (summary.rain_24h + recent.recent_rain)::REAL,
        (summary.rain_48h + recent.recent_rain)::REAL,
        (summary.rain_72h + recent.recent_rain)::REAL
    FROM summary, recent;
END;
$$ LANGUAGE plpgsql;

-- Restore original today_rainfall view
DROP VIEW IF EXISTS today_rainfall;

CREATE VIEW today_rainfall AS
SELECT
    COALESCE(SUM(period_rain), 0) +
    (SELECT COALESCE(SUM(rainincremental), 0)
     FROM weather
     WHERE time >= (SELECT max(bucket) FROM weather_5m)) AS total_rain
FROM weather_5m
WHERE bucket >= date_trunc('day', now());

-- Drop indexes created by this migration
DROP INDEX IF EXISTS rainfall_summary_stationname_idx;
DROP INDEX IF EXISTS weather_time_stationname_idx;