-- Reverse migration to remove HTTP/HTTPS server configuration from weather websites
-- (preserving data during rollback)

-- Step 1: Remove hostname index
DROP INDEX IF EXISTS idx_weather_websites_hostname;

-- Step 2: Add back snow_base_distance to weather_websites 
ALTER TABLE weather_websites ADD COLUMN snow_base_distance REAL;

-- Step 3: Migrate snow_base_distance back from devices to weather_websites
UPDATE weather_websites 
SET snow_base_distance = (
    SELECT d.snow_base_distance 
    FROM devices d 
    WHERE d.website_id = weather_websites.id 
    AND d.snow_base_distance IS NOT NULL
    ORDER BY d.id LIMIT 1  -- In case multiple devices, take the first one
)
WHERE EXISTS (
    SELECT 1 FROM devices d 
    WHERE d.website_id = weather_websites.id 
    AND d.snow_base_distance IS NOT NULL
);

-- Step 4: Remove snow_base_distance from devices (data now back in weather_websites)
ALTER TABLE devices DROP COLUMN snow_base_distance;

-- Step 5: Remove server configuration columns from weather_websites table
ALTER TABLE weather_websites DROP COLUMN hostname;
ALTER TABLE weather_websites DROP COLUMN tls_cert_path;
ALTER TABLE weather_websites DROP COLUMN tls_key_path; 