-- Add critical indexes to continuous aggregates for fast lookups
-- These are essential for the /latest endpoint performance

-- Index for weather_1m to speed up station+time queries
CREATE INDEX IF NOT EXISTS weather_1m_stationname_bucket_idx 
ON weather_1m (stationname, bucket DESC);

-- Index for weather_5m (used by today_rainfall and other views)
CREATE INDEX IF NOT EXISTS weather_5m_stationname_bucket_idx 
ON weather_5m (stationname, bucket DESC);

-- Index for weather_1h
CREATE INDEX IF NOT EXISTS weather_1h_stationname_bucket_idx 
ON weather_1h (stationname, bucket DESC);

-- Index for weather_1d  
CREATE INDEX IF NOT EXISTS weather_1d_stationname_bucket_idx
ON weather_1d (stationname, bucket DESC);

-- Optimize the today_rainfall view to be even more efficient
DROP VIEW IF EXISTS today_rainfall;

CREATE VIEW today_rainfall AS
SELECT 
    COALESCE(
        (SELECT SUM(period_rain) 
         FROM weather_5m 
         WHERE bucket >= date_trunc('day', now())
         LIMIT 1), 
        0
    ) + 
    COALESCE(
        (SELECT SUM(rainincremental) 
         FROM weather 
         WHERE time >= GREATEST(
             date_trunc('day', now()),
             (SELECT COALESCE(MAX(bucket), date_trunc('day', now())) 
              FROM weather_5m 
              LIMIT 1)
         )
         LIMIT 1), 
        0
    ) AS total_rain;

-- Analyze tables to update statistics for query planner
ANALYZE weather_1m;
ANALYZE weather_5m;
ANALYZE weather_1h;
ANALYZE weather_1d;
ANALYZE weather;
ANALYZE rainfall_summary;