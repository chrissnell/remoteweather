-- Create almanac cache table to store pre-computed all-time weather extremes
CREATE TABLE IF NOT EXISTS almanac_cache (
    stationname TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    wind_dir REAL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (stationname, metric_name)
);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_almanac_cache_stationname ON almanac_cache (stationname);

-- Create function to refresh almanac data for a specific station
-- This uses weather_1d materialized view which is MUCH faster than raw weather table
-- Function signature includes job_id and config for TimescaleDB job compatibility
CREATE OR REPLACE FUNCTION refresh_almanac_cache(p_stationname TEXT DEFAULT NULL, job_id INT DEFAULT NULL, config JSONB DEFAULT NULL)
RETURNS void AS $$
DECLARE
    station_rec RECORD;
BEGIN
    -- If station name provided, refresh just that station
    IF p_stationname IS NOT NULL THEN
        PERFORM refresh_almanac_cache_single(p_stationname);
        RETURN;
    END IF;

    -- Otherwise refresh all stations
    FOR station_rec IN
        SELECT DISTINCT stationname FROM weather WHERE stationname IS NOT NULL
    LOOP
        PERFORM refresh_almanac_cache_single(station_rec.stationname);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Internal function to refresh a single station (keeps main function clean)
CREATE OR REPLACE FUNCTION refresh_almanac_cache_single(p_stationname TEXT)
RETURNS void AS $$
BEGIN
    -- Delete existing data for this station
    DELETE FROM almanac_cache WHERE stationname = p_stationname;

    -- Highest temperature - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_temp', max_outtemp, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_outtemp IS NOT NULL
    ORDER BY max_outtemp DESC NULLS LAST
    LIMIT 1;

    -- Lowest temperature - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_temp', min_outtemp, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_outtemp IS NOT NULL
    ORDER BY min_outtemp ASC NULLS LAST
    LIMIT 1;

    -- Highest wind speed - use weather_1d for speed (note: winddir is circular_avg per day, not exact)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp, wind_dir)
    SELECT p_stationname, 'high_wind_speed', max_windspeed, bucket, winddir
    FROM weather_1d
    WHERE stationname = p_stationname AND max_windspeed IS NOT NULL
    ORDER BY max_windspeed DESC NULLS LAST
    LIMIT 1;

    -- Max rain in 1 hour
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'max_rain_hour', period_rain, bucket
    FROM weather_1h
    WHERE stationname = p_stationname AND period_rain IS NOT NULL
    ORDER BY period_rain DESC NULLS LAST
    LIMIT 1;

    -- Max rain in 1 day
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'max_rain_day', period_rain, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND period_rain IS NOT NULL
    ORDER BY period_rain DESC NULLS LAST
    LIMIT 1;

    -- Lowest barometer - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_barometer', min_barometer, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_barometer > 0
    ORDER BY min_barometer ASC NULLS LAST
    LIMIT 1;

    -- Lowest humidity - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_humidity', min_outhumidity, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_outhumidity > 0
    ORDER BY min_outhumidity ASC NULLS LAST
    LIMIT 1;

    -- Highest PM2.5 - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25', max_pm25, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25 > 0
    ORDER BY max_pm25 DESC NULLS LAST
    LIMIT 1;

    -- Highest PM10 - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm10_in', max_pm25_in, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25_in > 0
    ORDER BY max_pm25_in DESC NULLS LAST
    LIMIT 1;

    -- Highest CO2 - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_co2', max_co2, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_co2 > 0
    ORDER BY max_co2 DESC NULLS LAST
    LIMIT 1;

    -- Highest AQI PM2.5 - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_aqi_pm25', max_aqi_pm25_in::real, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_aqi_pm25_in > 0
    ORDER BY max_aqi_pm25_in DESC NULLS LAST
    LIMIT 1;

    -- Note: Snow metrics are calculated separately as they require base_distance parameter
    -- Note: Solar radiation and AQI PM10 excluded (not in weather_1d or too slow to query)
    -- These will be calculated on-demand in the application layer

END;
$$ LANGUAGE plpgsql;

-- Create a TimescaleDB job to refresh almanac cache every hour
-- Uses TimescaleDB's built-in job scheduler (not pg_cron)
DO $$
BEGIN
    -- Check if job already exists before creating
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE proc_name = 'refresh_almanac_cache'
    ) THEN
        PERFORM add_job(
            'refresh_almanac_cache',
            '1 hour',
            initial_start => NOW()
        );
    END IF;
END $$;

-- Perform initial population
SELECT refresh_almanac_cache();
