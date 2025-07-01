-- Migration to refactor APRS from global storage to per-station configuration
-- This preserves existing APRS data while moving it to the new structure
-- APRS server is global (controller), callsign/location are per-station

-- Step 1: Add APRS server field to controller_configs table for global APRS-IS server
ALTER TABLE controller_configs ADD COLUMN aprs_server TEXT;

-- Step 2: Create per-station APRS configuration table
-- Only callsign and location are per-station; passcode is calculated from callsign
CREATE TABLE station_aprs_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    device_name TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    callsign TEXT,
    latitude REAL,
    longitude REAL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    FOREIGN KEY (device_name) REFERENCES devices(name) ON DELETE CASCADE,
    UNIQUE(config_id, device_name)
);

-- Step 3: Migrate global APRS server config to controller_configs
INSERT INTO controller_configs (
    config_id, 
    controller_type, 
    enabled,
    aprs_server
)
SELECT 
    config_id,
    'aprs' as controller_type,
    enabled,
    COALESCE(aprs_server, 'noam.aprs2.net:14580')
FROM storage_configs 
WHERE backend_type = 'aprs' AND aprs_callsign IS NOT NULL;

-- Step 4: Create a station APRS config for each device, using the existing per-station data
-- This migrates the callsign and location to per-station configs
INSERT INTO station_aprs_configs (config_id, device_name, enabled, callsign, latitude, longitude)
SELECT d.config_id, d.name, sc.enabled, sc.aprs_callsign, sc.aprs_location_lat, sc.aprs_location_lon
FROM devices d
CROSS JOIN storage_configs sc
WHERE sc.backend_type = 'aprs' 
  AND sc.aprs_callsign IS NOT NULL
  AND d.config_id = sc.config_id;

-- Step 5: Remove APRS entries from storage_configs (data now split between controller_configs and station_aprs_configs)
DELETE FROM storage_configs WHERE backend_type = 'aprs';

-- Step 6: Add indexes for performance
CREATE INDEX idx_station_aprs_configs_device_name ON station_aprs_configs(device_name);
CREATE INDEX idx_station_aprs_configs_enabled ON station_aprs_configs(enabled); 