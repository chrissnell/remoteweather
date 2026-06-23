-- Almanac AQI is derived in the application (GetAlmanac) from the PM extremes
-- using the same pkg/aqi functions the live dashboard uses, so the cache stores
-- only measured quantities — never a computed AQI. This replaces the refresh so
-- it stops writing high_aqi_* rows and instead records the indoor PM2.5 max
-- (high_pm25_in) the handler needs to derive the indoor AQI card.

-- Drop the SQL AQI helpers if an earlier iteration created them; the EPA formula
-- lives only in pkg/aqi now.
DROP FUNCTION IF EXISTS calculate_aqi_pm25(real);
DROP FUNCTION IF EXISTS calculate_aqi_pm10(real);

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

    -- Highest solar radiation - sourced from the raw weather table (the
    -- continuous aggregates keep only a daily average of solarwatts). Runs in
    -- the hourly background job, never on a page request.
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_solar', solarwatts, time
    FROM weather
    WHERE stationname = p_stationname AND solarwatts IS NOT NULL AND solarwatts > 0
    ORDER BY solarwatts DESC
    LIMIT 1;

    -- Highest PM2.5 (outdoor)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25', max_pm25, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25 > 0
    ORDER BY max_pm25 DESC NULLS LAST
    LIMIT 1;

    -- Highest indoor PM2.5 (not shown directly; used to derive the indoor AQI card)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25_in', max_pm25_in, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25_in > 0
    ORDER BY max_pm25_in DESC NULLS LAST
    LIMIT 1;

    -- Highest PM10 - sourced from the AQIN sensor (the only PM10 reading we record)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm10_in', max_pm10_in_aqin, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm10_in_aqin > 0
    ORDER BY max_pm10_in_aqin DESC NULLS LAST
    LIMIT 1;

    -- Highest CO2
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_co2', max_co2, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_co2 > 0
    ORDER BY max_co2 DESC NULLS LAST
    LIMIT 1;

    -- Note: AQI extremes (high_aqi_pm25 / high_aqi_pm10 / high_aqi_pm25_in) are
    -- derived in GetAlmanac from the PM extremes above via pkg/aqi.
    -- Note: Snow metrics are calculated separately as they require base_distance parameter

END;
$$ LANGUAGE plpgsql;

-- Repopulate the cache so stale high_aqi_* rows are removed and high_pm25_in is added.
SELECT refresh_almanac_cache();
