-- Revert is_portal column back to is_dashboard
ALTER TABLE weather_websites RENAME COLUMN is_portal TO is_dashboard; 