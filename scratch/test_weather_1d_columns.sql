-- Test which columns exist in weather_1d and test query performance
\timing on

-- First, check what columns actually exist in weather_1d
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'weather_1d'
AND column_name LIKE '%max%' OR column_name LIKE '%min%'
ORDER BY column_name;

-- Test queries using weather_1d (should be FAST - milliseconds)
EXPLAIN ANALYZE
SELECT max_outtemp, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_outtemp IS NOT NULL
ORDER BY max_outtemp DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT min_outtemp, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND min_outtemp IS NOT NULL
ORDER BY min_outtemp ASC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_windspeed, bucket, winddir
FROM weather_1d
WHERE stationname = 'CSI' AND max_windspeed IS NOT NULL
ORDER BY max_windspeed DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT min_barometer, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND min_barometer > 0
ORDER BY min_barometer ASC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT min_outhumidity, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND min_outhumidity > 0
ORDER BY min_outhumidity ASC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_pm25, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_pm25 > 0
ORDER BY max_pm25 DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_pm25_in, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_pm25_in > 0
ORDER BY max_pm25_in DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_co2, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_co2 > 0
ORDER BY max_co2 DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_aqi_pm25_in, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_aqi_pm25_in > 0
ORDER BY max_aqi_pm25_in DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT max_aqi_pm10_in, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND max_aqi_pm10_in > 0
ORDER BY max_aqi_pm10_in DESC NULLS LAST
LIMIT 1;

EXPLAIN ANALYZE
SELECT period_rain, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND period_rain IS NOT NULL
ORDER BY period_rain DESC NULLS LAST
LIMIT 1;

-- Test the hybrid solar query (should still be fast - only checks 30 days)
EXPLAIN ANALYZE
WITH top_solar_days AS (
    SELECT bucket::date as day
    FROM weather_1d
    WHERE stationname = 'CSI' AND solarwatts IS NOT NULL
    ORDER BY solarwatts DESC NULLS LAST
    LIMIT 30
)
SELECT w.solarwatts, w.time
FROM weather w
INNER JOIN top_solar_days tsd ON w.time::date = tsd.day
WHERE w.stationname = 'CSI' AND w.solarwatts IS NOT NULL
ORDER BY w.solarwatts DESC NULLS LAST
LIMIT 1;

-- Check table sizes
SELECT
    'weather' as table_name,
    pg_size_pretty(pg_total_relation_size('weather')) AS size,
    (SELECT COUNT(*) FROM weather WHERE stationname = 'CSI') as row_count
UNION ALL
SELECT
    'weather_1d' as table_name,
    pg_size_pretty(pg_total_relation_size('weather_1d')) AS size,
    (SELECT COUNT(*) FROM weather_1d WHERE stationname = 'CSI') as row_count;
