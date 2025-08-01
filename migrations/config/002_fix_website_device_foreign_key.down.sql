-- Migration 002 Rollback: Revert weather website device association to use device names
-- This converts back from device IDs to device names

-- Create table with old structure (device_id as TEXT)
CREATE TABLE weather_websites_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id TEXT, -- Back to storing device names
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    tls_cert_path TEXT,
    tls_key_path TEXT,
    is_dashboard BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Copy data back, converting device IDs to device names
INSERT INTO weather_websites_old (
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, tls_cert_path, tls_key_path, is_dashboard,
    created_at, updated_at
)
SELECT 
    w.id, w.config_id, w.name, 
    COALESCE(d.name, '') as device_id, -- Convert device ID back to device name
    w.hostname, w.page_title, w.about_station_html,
    w.snow_enabled, w.snow_device_name, w.tls_cert_path, w.tls_key_path, w.is_dashboard,
    w.created_at, w.updated_at
FROM weather_websites w
LEFT JOIN devices d ON w.device_id = d.id;

-- Drop new table and rename old table
DROP TABLE weather_websites;
ALTER TABLE weather_websites_old RENAME TO weather_websites;

-- Recreate index
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id); 