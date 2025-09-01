-- Recreate the today_rainfall view (for rollback compatibility)
-- Note: This view is inefficient and not used anymore
CREATE VIEW IF NOT EXISTS today_rainfall AS
SELECT 
    COALESCE(
        (SELECT SUM(period_rain) 
         FROM weather_5m 
         WHERE bucket >= date_trunc('day', now())
         LIMIT 1), 
        0
    ) + 
    COALESCE(
        (SELECT SUM(rainincremental) 
         FROM weather 
         WHERE time >= GREATEST(
             date_trunc('day', now()),
             (SELECT COALESCE(MAX(bucket), date_trunc('day', now())) 
              FROM weather_5m 
              LIMIT 1)
         )
         LIMIT 1), 
        0
    ) AS total_rain;