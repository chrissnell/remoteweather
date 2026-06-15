-- Add live weather radar support to weather websites.
ALTER TABLE weather_websites ADD COLUMN radar_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE weather_websites ADD COLUMN radar_token TEXT DEFAULT '';
ALTER TABLE weather_websites ADD COLUMN radar_registered_at INTEGER;
