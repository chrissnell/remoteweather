-- Add air quality support to weather websites
ALTER TABLE weather_websites ADD COLUMN air_quality_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE weather_websites ADD COLUMN air_quality_device_name TEXT;