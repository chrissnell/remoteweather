-- Rollback migration 012: Remove weather service fields from devices table
-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table

-- Create a temporary table without the new columns
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
    base_snow_distance REAL,
    website_id INTEGER,
    latitude REAL,
    longitude REAL,
    altitude REAL,
    aprs_enabled BOOLEAN DEFAULT FALSE,
    aprs_callsign TEXT,
    tls_cert_file TEXT,
    tls_key_file TEXT,
    path TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Copy data from the current table to the temporary table
INSERT INTO devices_temp (
    id, config_id, name, type, enabled, hostname, port, serial_device, baud,
    wind_dir_correction, base_snow_distance, website_id, latitude, longitude,
    altitude, aprs_enabled, aprs_callsign, tls_cert_file, tls_key_file, path,
    created_at, updated_at
)
SELECT 
    id, config_id, name, type, enabled, hostname, port, serial_device, baud,
    wind_dir_correction, base_snow_distance, website_id, latitude, longitude,
    altitude, aprs_enabled, aprs_callsign, tls_cert_file, tls_key_file, path,
    created_at, updated_at
FROM devices;

-- Drop the original table
DROP TABLE devices;

-- Rename the temporary table to the original table name
ALTER TABLE devices_temp RENAME TO devices;

-- Recreate original indexes
CREATE INDEX idx_devices_config_id ON devices(config_id);
CREATE INDEX idx_devices_name ON devices(config_id, name);

-- Drop the weather service indexes (they'll be gone with the table drop above, but listed for completeness)
-- DROP INDEX IF EXISTS idx_devices_pws_enabled;
-- DROP INDEX IF EXISTS idx_devices_wu_enabled;
-- DROP INDEX IF EXISTS idx_devices_aeris_enabled;