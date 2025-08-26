-- Remove air quality support from weather websites
-- Note: SQLite doesn't support DROP COLUMN, so we recreate the table without these fields
-- This is a destructive operation that will lose data in these columns

-- Create temporary table without air quality columns
CREATE TABLE weather_websites_temp (
    id INTEGER PRIMARY KEY,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id INTEGER,
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    tls_cert_path TEXT,
    tls_key_path TEXT,
    is_portal BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (config_id) REFERENCES configs(id),
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- Copy data from old table (excluding air quality columns)
INSERT INTO weather_websites_temp (
    id, config_id, name, device_id, hostname, page_title, 
    about_station_html, snow_enabled, snow_device_name,
    tls_cert_path, tls_key_path, is_portal
)
SELECT 
    id, config_id, name, device_id, hostname, page_title,
    about_station_html, snow_enabled, snow_device_name,
    tls_cert_path, tls_key_path, is_portal
FROM weather_websites;

-- Drop old table
DROP TABLE weather_websites;

-- Rename temp table to original name
ALTER TABLE weather_websites_temp RENAME TO weather_websites;