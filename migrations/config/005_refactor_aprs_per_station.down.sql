-- Reverse migration to restore APRS to global storage configuration
-- This preserves APRS data while moving it back to the original structure

-- Step 1: Remove indexes
DROP INDEX IF EXISTS idx_station_aprs_configs_device_name;
DROP INDEX IF EXISTS idx_station_aprs_configs_enabled;

-- Step 2: Restore APRS data to storage_configs by combining controller and station configs
-- (Since the original design was global, we take the first station's APRS config for callsign/location)
INSERT INTO storage_configs (
    config_id, backend_type, enabled,
    aprs_callsign, aprs_passcode, aprs_server, aprs_location_lat, aprs_location_lon
)
SELECT DISTINCT 
    cc.config_id, 
    'aprs' as backend_type,
    cc.enabled,
    sa.callsign,
    '', -- passcode will need to be recalculated
    cc.aprs_server,
    sa.latitude,
    sa.longitude
FROM controller_configs cc
LEFT JOIN station_aprs_configs sa ON cc.config_id = sa.config_id
WHERE cc.controller_type = 'aprs' AND sa.enabled = 1
GROUP BY cc.config_id  -- Take first station per config in case of multiple
HAVING MIN(sa.id);     -- Use the first station that was added

-- Step 3: Remove APRS controller entries
DELETE FROM controller_configs WHERE controller_type = 'aprs';

-- Step 4: Remove APRS server field from controller_configs table
ALTER TABLE controller_configs DROP COLUMN aprs_server;

-- Step 5: Drop the station APRS table
DROP TABLE station_aprs_configs; 