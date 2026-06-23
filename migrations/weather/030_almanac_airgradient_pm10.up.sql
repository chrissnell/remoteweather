-- PM10 lives in different columns depending on the air quality device:
--   * Ambient AQIN sensors report it as pm10_in_aqin
--   * AirGradient sensors report it in extrafloat2 (see the airgradient station
--     mapping, which packs PM10/PM1/TVOC/NOx into the extrafloat fields)
-- The almanac only read pm10_in_aqin, so PM10 (and the derived AQI PM10) were
-- always blank for AirGradient devices. Source PM10 per station type.
--
-- Also drops high_pm25_in: the indoor AQI PM2.5 card has been removed, so the
-- indoor PM2.5 max is no longer needed.
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

    -- Highest solar radiation - from the raw weather table (aggregates keep only a daily average)
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

    -- Highest PM10 - AirGradient stores it in extrafloat2; Ambient AQIN in pm10_in_aqin.
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_pm10_in', val, bucket
    FROM (
        SELECT bucket,
               CASE WHEN stationtype = 'airgradient' THEN max_extrafloat2
                    ELSE max_pm10_in_aqin END AS val
        FROM weather_1d
        WHERE stationname = p_stationname
    ) s
    WHERE val > 0
    ORDER BY val DESC NULLS LAST
    LIMIT 1;

    -- Highest CO2
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_co2', max_co2, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND max_co2 > 0
    ORDER BY max_co2 DESC NULLS LAST
    LIMIT 1;

    -- Highest TVOC index - AirGradient only (stored in extrafloat3)
    INSERT INTO almanac_cache (stationname, metric_name, value, timestamp)
    SELECT p_stationname, 'high_tvoc', max_extrafloat3, bucket
    FROM weather_1d
    WHERE stationname = p_stationname AND stationtype = 'airgradient' AND max_extrafloat3 > 0
    ORDER BY max_extrafloat3 DESC NULLS LAST
    LIMIT 1;

    -- Note: AQI extremes (high_aqi_pm25 / high_aqi_pm10) are derived in
    -- GetAlmanac from the PM extremes above via pkg/aqi.
    -- Note: Snow metrics are calculated separately as they require base_distance parameter

END;
$$ LANGUAGE plpgsql;

-- Repopulate so PM10 fills in for AirGradient devices and high_pm25_in is dropped.
SELECT refresh_almanac_cache();
