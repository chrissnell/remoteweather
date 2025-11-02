-- Add partial indexes for snow-related almanac queries
-- These optimize the window function queries for max snow in 1h/1d

-- Index for weather_1h snow queries (max snow in 1 hour)
CREATE INDEX IF NOT EXISTS weather_1h_stationname_snowdistance_bucket_idx
ON weather_1h (stationname, bucket DESC)
WHERE snowdistance IS NOT NULL AND snowdistance > 0;

-- Index for weather_1d snow queries (max snow in 1 day)
CREATE INDEX IF NOT EXISTS weather_1d_stationname_snowdistance_bucket_idx
ON weather_1d (stationname, bucket DESC)
WHERE snowdistance IS NOT NULL AND snowdistance > 0;

-- Analyze tables to update statistics for query planner
ANALYZE weather_1h;
ANALYZE weather_1d;
