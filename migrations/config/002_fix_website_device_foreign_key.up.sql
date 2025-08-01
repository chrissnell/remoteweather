-- Migration 002: Fix weather website device association to use proper foreign keys
-- Instead of storing device names, store device IDs with proper foreign key constraints

-- Step 1: Add new column for device ID reference
ALTER TABLE weather_websites ADD COLUMN device_ref_id INTEGER;

-- Step 2: Populate the new column by looking up device IDs from device names
UPDATE weather_websites 
SET device_ref_id = (
    SELECT d.id 
    FROM devices d 
    WHERE d.name = weather_websites.device_id 
    AND d.config_id = weather_websites.config_id
)
WHERE device_id IS NOT NULL AND device_id != '';

-- Step 3: Add foreign key constraint
-- Note: SQLite doesn't support ADD CONSTRAINT, so we need to recreate the table

-- Create new table with proper structure
CREATE TABLE weather_websites_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id INTEGER, -- Now references devices.id instead of device name
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT, -- This could also be converted to FK, but keeping for now
    tls_cert_path TEXT,
    tls_key_path TEXT,
    is_dashboard BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL,
    UNIQUE(config_id, name)
);

-- Copy data from old table to new table (using device_ref_id as the new device_id)
INSERT INTO weather_websites_new (
    id, config_id, name, device_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, tls_cert_path, tls_key_path, is_dashboard,
    created_at, updated_at
)
SELECT 
    id, config_id, name, device_ref_id, hostname, page_title, about_station_html,
    snow_enabled, snow_device_name, tls_cert_path, tls_key_path, is_dashboard,
    created_at, updated_at
FROM weather_websites;

-- Drop old table and rename new table
DROP TABLE weather_websites;
ALTER TABLE weather_websites_new RENAME TO weather_websites;

-- Recreate indexes
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id);
CREATE INDEX idx_weather_websites_device_id ON weather_websites(device_id); 