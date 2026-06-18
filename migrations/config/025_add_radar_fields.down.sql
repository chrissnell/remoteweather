-- Remove live weather radar support. SQLite has no DROP COLUMN, so rebuild the
-- table without the radar columns, preserving everything migrations 002-021 left.
CREATE TABLE weather_websites_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id INTEGER,
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    air_quality_enabled BOOLEAN DEFAULT FALSE,
    air_quality_device_name TEXT,
    tls_cert_path TEXT,
    tls_key_path TEXT,
    is_portal BOOLEAN DEFAULT FALSE,
    apple_app_id TEXT DEFAULT '6755874087',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL,
    UNIQUE(config_id, name)
);
INSERT INTO weather_websites_temp (
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, air_quality_enabled, air_quality_device_name,
    tls_cert_path, tls_key_path, is_portal, apple_app_id, created_at, updated_at
)
SELECT
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, air_quality_enabled, air_quality_device_name,
    tls_cert_path, tls_key_path, is_portal, apple_app_id, created_at, updated_at
FROM weather_websites;
DROP TABLE weather_websites;
ALTER TABLE weather_websites_temp RENAME TO weather_websites;
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id);
CREATE INDEX idx_weather_websites_device_id ON weather_websites(device_id);
