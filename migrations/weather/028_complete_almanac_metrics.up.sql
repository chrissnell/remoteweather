-- Complete the almanac cache so every metric the UI renders is populated.
--
-- The original refresh_almanac_cache_single (migration 005) was missing three
-- metrics that the frontend always displayed as "--" (high_solar, high_aqi_pm10,
-- high_aqi_pm25_in) and mapped two others to the wrong source columns
-- (high_pm10_in read indoor PM2.5, high_aqi_pm25 read the indoor AQI). This
-- replaces the function so all extremes are pre-computed by the hourly job and
-- read from almanac_cache on page load (no per-request queries against the
-- weather table).

-- Partial index so the high_solar lookup is an indexed top-1 scan instead of a
-- full table scan. Mirrors the existing weather_stationname_snowdistance_time_idx
-- pattern used for the snow almanac queries. The solar peak is the only metric
-- sourced from the raw weather table (the continuous aggregates keep only a
-- daily average of solarwatts), so this keeps the hourly refresh job cheap.
CREATE INDEX IF NOT EXISTS weather_stationname_solarwatts_idx
    ON weather (stationname, solarwatts DESC)
    WHERE solarwatts IS NOT NULL AND solarwatts > 0;

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

    -- Highest solar radiation - the continuous aggregates only keep a daily
    -- average of solarwatts, so the all-time peak has to come from the raw
    -- weather table. This is the only raw-table scan in the function and it
    -- runs in the hourly background job, never on a page request.
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_solar', solarwatts, time
    FROM weather
    WHERE stationname = p_stationname AND solarwatts IS NOT NULL AND solarwatts > 0
    ORDER BY solarwatts DESC
    LIMIT 1;

    -- Highest PM2.5 (outdoor) - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25', max_pm25, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25 > 0
    ORDER BY max_pm25 DESC NULLS LAST
    LIMIT 1;

    -- Highest PM10 - sourced from the AQIN sensor (the only PM10 reading we
    -- record). Previously this read max_pm25_in (indoor PM2.5) by mistake.
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm10_in', max_pm10_in_aqin, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm10_in_aqin > 0
    ORDER BY max_pm10_in_aqin DESC NULLS LAST
    LIMIT 1;

    -- Highest CO2 - use weather_1d for speed
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_co2', max_co2, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_co2 > 0
    ORDER BY max_co2 DESC NULLS LAST
    LIMIT 1;

    -- Highest AQI PM2.5 - the AQIN sensor's outdoor AQI, matching the live
    -- display. Previously this read the indoor AQI (max_aqi_pm25_in).
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_aqi_pm25', max_aqi_pm25_aqin::real, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_aqi_pm25_aqin > 0
    ORDER BY max_aqi_pm25_aqin DESC NULLS LAST
    LIMIT 1;

    -- Highest AQI PM10 (AQIN sensor)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_aqi_pm10', max_aqi_pm10_aqin::real, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_aqi_pm10_aqin > 0
    ORDER BY max_aqi_pm10_aqin DESC NULLS LAST
    LIMIT 1;

    -- Highest indoor AQI PM2.5 (console sensor)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_aqi_pm25_in', max_aqi_pm25_in::real, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_aqi_pm25_in > 0
    ORDER BY max_aqi_pm25_in DESC NULLS LAST
    LIMIT 1;

    -- Note: Snow metrics are calculated separately as they require base_distance parameter

END;
$$ LANGUAGE plpgsql;

-- Repopulate the cache for every station with the corrected/expanded metrics.
SELECT refresh_almanac_cache();
