-- Remove API endpoint fields from devices table
-- SQLite doesn't support dropping columns directly, so we need to recreate the table

-- Create a temporary table with the old structure
CREATE TABLE devices_temp AS 
SELECT id, config_id, name, type, serial_device, hostname, port, enabled, 
       location_description, latitude, longitude, base_snow_distance, path,
       pws_enabled, pws_station_id, pws_password, pws_upload_interval,
       wu_enabled, wu_station_id, wu_password, wu_upload_interval,
       aeris_enabled, aeris_api_client_id, aeris_api_client_secret,
       aprs_enabled, aprs_callsign, aprs_passcode
FROM devices;

-- Drop the current table
DROP TABLE devices;

-- Recreate the table without the API endpoint columns
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    serial_device TEXT,
    hostname TEXT,
    port TEXT,
    enabled BOOLEAN DEFAULT 1,
    location_description TEXT,
    latitude REAL,
    longitude REAL,
    base_snow_distance INTEGER,
    path TEXT,
    -- PWS Weather fields
    pws_enabled BOOLEAN DEFAULT 0,
    pws_station_id TEXT,
    pws_password TEXT,
    pws_upload_interval INTEGER,
    -- Weather Underground fields
    wu_enabled BOOLEAN DEFAULT 0,
    wu_station_id TEXT,
    wu_password TEXT,
    wu_upload_interval INTEGER,
    -- Aeris Weather fields
    aeris_enabled BOOLEAN DEFAULT 0,
    aeris_api_client_id TEXT,
    aeris_api_client_secret TEXT,
    -- APRS fields
    aprs_enabled BOOLEAN DEFAULT 0,
    aprs_callsign TEXT,
    aprs_passcode TEXT,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE
);

-- Copy data back
INSERT INTO devices SELECT * FROM devices_temp;

-- Drop temporary table
DROP TABLE devices_temp;

-- Recreate indexes
CREATE INDEX idx_devices_config_id ON devices(config_id);
CREATE INDEX idx_devices_name ON devices(name);
CREATE INDEX idx_devices_type ON devices(type);