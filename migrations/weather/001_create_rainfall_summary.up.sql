-- First migration for TimescaleDB weather database
-- Create migration tracking table if it doesn't exist
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Create rainfall summary table for fast queries
CREATE TABLE IF NOT EXISTS rainfall_summary (
    stationname TEXT PRIMARY KEY,
    rain_24h REAL DEFAULT 0,
    rain_48h REAL DEFAULT 0,
    rain_72h REAL DEFAULT 0,
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast lookups
CREATE INDEX IF NOT EXISTS idx_rainfall_summary_stationname 
ON rainfall_summary (stationname);

-- Function to update rainfall summary using hierarchical aggregates
CREATE OR REPLACE FUNCTION update_rainfall_summary()
RETURNS void AS $$
DECLARE
    last_hour TIMESTAMPTZ;
    last_5min TIMESTAMPTZ;
    cutoff_24h TIMESTAMPTZ;
    cutoff_48h TIMESTAMPTZ;
    cutoff_72h TIMESTAMPTZ;
BEGIN
    -- Calculate time boundaries
    last_hour := date_trunc('hour', NOW());
    last_5min := date_trunc('hour', NOW()) + 
        (EXTRACT(MINUTE FROM NOW())::INT / 5 * 5 || ' minutes')::INTERVAL;
    cutoff_24h := NOW() - INTERVAL '24 hours';
    cutoff_48h := NOW() - INTERVAL '48 hours';
    cutoff_72h := NOW() - INTERVAL '72 hours';
    
    -- Update all active stations using hierarchical aggregates
    INSERT INTO rainfall_summary (stationname, rain_24h, rain_48h, rain_72h, last_updated)
    SELECT 
        s.stationname,
        -- 24-hour rainfall: hourly + 5min + recent
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_24h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_24h,
        
        -- 48-hour rainfall
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_48h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_48h,
        
        -- 72-hour rainfall
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_72h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_72h,
        
        NOW()
    FROM (
        -- Only update stations that have recent activity
        SELECT DISTINCT stationname 
        FROM weather 
        WHERE time >= NOW() - INTERVAL '10 minutes'
    ) s
    ON CONFLICT (stationname) DO UPDATE SET
        rain_24h = EXCLUDED.rain_24h,
        rain_48h = EXCLUDED.rain_48h,
        rain_72h = EXCLUDED.rain_72h,
        last_updated = EXCLUDED.last_updated;
END;
$$ LANGUAGE plpgsql;

-- Create function for getting rainfall with recent updates
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

-- Create TimescaleDB background job to update summary every minute
DO $$
BEGIN
    -- Check if job already exists before creating
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs 
        WHERE proc_name = 'update_rainfall_summary'
    ) THEN
        PERFORM add_job(
            'update_rainfall_summary',
            '1 minute',
            initial_start => NOW()
        );
    END IF;
END $$;

-- Initialize the summary table with current data
SELECT update_rainfall_summary();