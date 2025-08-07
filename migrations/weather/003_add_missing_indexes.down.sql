-- Remove indexes added by this migration
DROP INDEX IF EXISTS weather_1m_stationname_bucket_idx;
DROP INDEX IF EXISTS weather_5m_stationname_bucket_idx;
DROP INDEX IF EXISTS weather_1h_stationname_bucket_idx;
DROP INDEX IF EXISTS weather_1d_stationname_bucket_idx;

-- Restore original today_rainfall view
DROP VIEW IF EXISTS today_rainfall;

CREATE VIEW today_rainfall AS
WITH recent_weather AS (
    SELECT COALESCE(SUM(rainincremental), 0) as recent_rain
    FROM weather
    WHERE time >= GREATEST(
        date_trunc('day', now()),
        (SELECT COALESCE(MAX(bucket), date_trunc('day', now())) FROM weather_5m)
    )
)
SELECT
    COALESCE(
        (SELECT SUM(period_rain) FROM weather_5m WHERE bucket >= date_trunc('day', now())),
        0
    ) + COALESCE((SELECT recent_rain FROM recent_weather), 0) AS total_rain;