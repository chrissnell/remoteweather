-- Recreate the alerts table (if rolling back)
CREATE TABLE IF NOT EXISTS aeris_weather_alerts (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    station_id INT NOT NULL,
    alert_id TEXT NOT NULL UNIQUE,
    location TEXT NOT NULL,
    issued_at TIMESTAMPTZ,
    begins_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    name TEXT,
    color TEXT,
    body TEXT,
    body_full TEXT,
    data JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alerts_station_id ON aeris_weather_alerts(station_id);
CREATE INDEX IF NOT EXISTS idx_alerts_expires_at ON aeris_weather_alerts(expires_at);
CREATE INDEX IF NOT EXISTS idx_alerts_alert_id ON aeris_weather_alerts(alert_id);
CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON aeris_weather_alerts(deleted_at);
