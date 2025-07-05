-- Rename is_dashboard column to is_portal for better naming
ALTER TABLE weather_websites RENAME COLUMN is_dashboard TO is_portal; 