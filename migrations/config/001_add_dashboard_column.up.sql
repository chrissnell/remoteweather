-- Add is_dashboard column to weather_websites table
ALTER TABLE weather_websites ADD COLUMN is_dashboard BOOLEAN DEFAULT FALSE; 