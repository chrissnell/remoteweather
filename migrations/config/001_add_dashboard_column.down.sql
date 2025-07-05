-- Remove is_dashboard column from weather_websites table
-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table

-- Create a temporary table without the is_dashboard column
CREATE TABLE weather_websites_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id TEXT,
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    tls_cert_path TEXT,
    tls_key_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Copy data from original table to temporary table
INSERT INTO weather_websites_temp (
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, tls_cert_path, tls_key_path, created_at, updated_at
)
SELECT 
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, tls_cert_path, tls_key_path, created_at, updated_at
FROM weather_websites;

-- Drop the original table
DROP TABLE weather_websites;

-- Rename the temporary table to the original name
ALTER TABLE weather_websites_temp RENAME TO weather_websites; 