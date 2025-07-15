-- Remove path field from devices table
-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table

-- Create a temporary table without the path column
CREATE TABLE devices_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    hostname TEXT,
    port TEXT,
    serial_device TEXT,
    baud INTEGER,
    wind_dir_correction INTEGER,
    base_snow_distance INTEGER,
    website_id INTEGER,
    latitude REAL,
    longitude REAL,
    altitude REAL,
    aprs_enabled BOOLEAN DEFAULT FALSE,
    aprs_callsign TEXT,
    tls_cert_file TEXT,
    tls_key_file TEXT,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Copy data from original table to temporary table
INSERT INTO devices_temp (
    id, config_id, name, type, enabled, hostname, port, serial_device,
    baud, wind_dir_correction, base_snow_distance, website_id,
    latitude, longitude, altitude, aprs_enabled, aprs_callsign,
    tls_cert_file, tls_key_file
)
SELECT 
    id, config_id, name, type, enabled, hostname, port, serial_device,
    baud, wind_dir_correction, base_snow_distance, website_id,
    latitude, longitude, altitude, aprs_enabled, aprs_callsign,
    tls_cert_file, tls_key_file
FROM devices;

-- Drop the original table
DROP TABLE devices;

-- Rename the temporary table to the original name
ALTER TABLE devices_temp RENAME TO devices;