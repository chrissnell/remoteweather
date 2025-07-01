-- Migration to add HTTP/HTTPS server configuration to weather websites
-- and move snow_base_distance back to devices where it belongs (preserving data)

-- Step 1: Add new server configuration columns to weather_websites table
ALTER TABLE weather_websites ADD COLUMN hostname TEXT;
ALTER TABLE weather_websites ADD COLUMN tls_cert_path TEXT;
ALTER TABLE weather_websites ADD COLUMN tls_key_path TEXT;

-- Step 2: Migrate snow_base_distance from weather_websites to devices
-- First, add snow_base_distance to devices if it doesn't exist
ALTER TABLE devices ADD COLUMN snow_base_distance REAL;

-- Update devices with snow_base_distance from their associated weather website
UPDATE devices 
SET snow_base_distance = (
    SELECT ww.snow_base_distance 
    FROM weather_websites ww 
    WHERE devices.website_id = ww.id
)
WHERE devices.website_id IS NOT NULL 
AND EXISTS (
    SELECT 1 FROM weather_websites ww 
    WHERE devices.website_id = ww.id 
    AND ww.snow_base_distance IS NOT NULL
);

-- Step 3: Remove snow_base_distance from weather_websites (data now preserved in devices)
ALTER TABLE weather_websites DROP COLUMN snow_base_distance;

-- Step 4: Add index for hostname lookups
CREATE INDEX idx_weather_websites_hostname ON weather_websites(hostname); 