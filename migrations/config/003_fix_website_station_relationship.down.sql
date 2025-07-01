-- Reverse migration to undo website-station relationship changes

-- Remove the website_id column from devices table
ALTER TABLE devices DROP COLUMN website_id;

-- Drop the weather_websites table
DROP TRIGGER IF EXISTS update_weather_websites_timestamp;
DROP INDEX IF EXISTS idx_weather_websites_config_id;
DROP INDEX IF EXISTS idx_devices_website_id;
DROP TABLE IF EXISTS weather_websites; 