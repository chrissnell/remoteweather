-- Test query performance for almanac cache generation
-- Run these queries to see timing and execution plans

-- First, enable timing and analyze
\timing on

-- Test highest temperature query
EXPLAIN ANALYZE
SELECT stationname, outtemp, time
FROM weather
WHERE stationname = 'CSI' AND outtemp IS NOT NULL
ORDER BY outtemp DESC NULLS LAST
LIMIT 1;

-- Test lowest temperature query
EXPLAIN ANALYZE
SELECT stationname, outtemp, time
FROM weather
WHERE stationname = 'CSI' AND outtemp IS NOT NULL
ORDER BY outtemp ASC NULLS LAST
LIMIT 1;

-- Test highest wind speed query
EXPLAIN ANALYZE
SELECT stationname, windspeed, time, winddir
FROM weather
WHERE stationname = 'CSI' AND windspeed IS NOT NULL
ORDER BY windspeed DESC NULLS LAST
LIMIT 1;

-- Test max rain hour query
EXPLAIN ANALYZE
SELECT stationname, period_rain, bucket
FROM weather_1h
WHERE stationname = 'CSI' AND period_rain IS NOT NULL
ORDER BY period_rain DESC NULLS LAST
LIMIT 1;

-- Test max rain day query
EXPLAIN ANALYZE
SELECT stationname, period_rain, bucket
FROM weather_1d
WHERE stationname = 'CSI' AND period_rain IS NOT NULL
ORDER BY period_rain DESC NULLS LAST
LIMIT 1;

-- Check table sizes to understand data volume
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    pg_total_relation_size(schemaname||'.'||tablename) AS size_bytes
FROM pg_tables
WHERE tablename IN ('weather', 'weather_1h', 'weather_1d')
ORDER BY size_bytes DESC;

-- Check row counts per station
SELECT
    stationname,
    COUNT(*) as row_count,
    MIN(time) as earliest,
    MAX(time) as latest
FROM weather
GROUP BY stationname
ORDER BY row_count DESC;

-- Test the OLD inefficient query pattern (the current one causing timeouts)
EXPLAIN ANALYZE
SELECT
    MAX(outtemp) as max_out_temp,
    (SELECT time FROM weather w WHERE w.outtemp = MAX(weather.outtemp) AND w.stationname = 'CSI' LIMIT 1) as time
FROM weather
WHERE stationname = 'CSI';

-- Compare: NEW efficient query pattern
EXPLAIN ANALYZE
SELECT outtemp as max_out_temp, time
FROM weather
WHERE stationname = 'CSI' AND outtemp IS NOT NULL
ORDER BY outtemp DESC NULLS LAST
LIMIT 1;
