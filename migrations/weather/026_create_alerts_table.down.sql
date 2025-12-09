-- Drop indexes
DROP INDEX IF EXISTS idx_alerts_deleted_at;
DROP INDEX IF EXISTS idx_alerts_alert_id;
DROP INDEX IF EXISTS idx_alerts_expires_at;
DROP INDEX IF EXISTS idx_alerts_station_id;

-- Drop table
DROP TABLE IF EXISTS aeris_weather_alerts;
