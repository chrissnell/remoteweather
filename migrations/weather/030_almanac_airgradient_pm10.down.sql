-- Revert to the 029 behavior: PM10 only from pm10_in_aqin, and re-add the
-- indoor PM2.5 max (high_pm25_in).
CREATE OR REPLACE FUNCTION refresh_almanac_cache_single(p_stationname TEXT)
RETURNS void AS $$
BEGIN
    DELETE FROM almanac_cache WHERE stationname = p_stationname;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_temp', max_outtemp, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_outtemp IS NOT NULL
    ORDER BY max_outtemp DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_temp', min_outtemp, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_outtemp IS NOT NULL
    ORDER BY min_outtemp ASC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp, wind_dir)
    SELECT p_stationname, 'high_wind_speed', max_windspeed, bucket, winddir
    FROM weather_1d
    WHERE stationname = p_stationname AND max_windspeed IS NOT NULL
    ORDER BY max_windspeed DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'max_rain_hour', period_rain, bucket
    FROM weather_1h
    WHERE stationname = p_stationname AND period_rain IS NOT NULL
    ORDER BY period_rain DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'max_rain_day', period_rain, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND period_rain IS NOT NULL
    ORDER BY period_rain DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_barometer', min_barometer, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_barometer > 0
    ORDER BY min_barometer ASC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'low_humidity', min_outhumidity, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND min_outhumidity > 0
    ORDER BY min_outhumidity ASC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_solar', solarwatts, time
    FROM weather
    WHERE stationname = p_stationname AND solarwatts IS NOT NULL AND solarwatts > 0
    ORDER BY solarwatts DESC
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25', max_pm25, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25 > 0
    ORDER BY max_pm25 DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm25_in', max_pm25_in, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm25_in > 0
    ORDER BY max_pm25_in DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm10_in', max_pm10_in_aqin, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_pm10_in_aqin > 0
    ORDER BY max_pm10_in_aqin DESC NULLS LAST
    LIMIT 1;

    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_co2', max_co2, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_co2 > 0
    ORDER BY max_co2 DESC NULLS LAST
    LIMIT 1;

END;
$$ LANGUAGE plpgsql;

SELECT refresh_almanac_cache();
