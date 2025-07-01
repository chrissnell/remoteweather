-- Migration to fix website-station relationship
-- Instead of websites referencing stations by name, stations will reference websites by ID

-- First, create a new weather_websites table with proper primary keys
CREATE TABLE weather_websites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    snow_base_distance REAL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Add website_id column to devices table
ALTER TABLE devices ADD COLUMN website_id INTEGER;
ALTER TABLE devices ADD FOREIGN KEY (website_id) REFERENCES weather_websites(id) ON DELETE SET NULL;

-- Create index for better query performance
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id);
CREATE INDEX idx_devices_website_id ON devices(website_id);

-- Trigger to update updated_at timestamp for weather_websites
CREATE TRIGGER update_weather_websites_timestamp 
    AFTER UPDATE ON weather_websites
    FOR EACH ROW
BEGIN
    UPDATE weather_websites SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Migration data: Create default website entries from existing weather_site_configs
-- This preserves existing configurations during the migration
INSERT INTO weather_websites (config_id, name, page_title, about_station_html, snow_enabled, snow_device_name, snow_base_distance)
SELECT 
    c.config_id,
    COALESCE(wsc.station_name, 'default-website') as name,
    wsc.page_title,
    wsc.about_station_html,
    wsc.snow_enabled,
    wsc.snow_device,
    wsc.snow_base_distance
FROM weather_site_configs wsc
JOIN controller_configs cc ON wsc.controller_config_id = cc.id
JOIN configs c ON cc.config_id = c.id;

-- Update devices to reference the newly created websites
-- We'll associate devices with websites based on matching pull_from_device
UPDATE devices 
SET website_id = (
    SELECT ww.id 
    FROM weather_websites ww
    JOIN configs c ON ww.config_id = c.config_id
    JOIN controller_configs cc ON c.id = cc.config_id
    JOIN weather_site_configs wsc ON cc.id = wsc.controller_config_id
    WHERE wsc.pull_from_device = devices.name
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1 
    FROM weather_websites ww
    JOIN configs c ON ww.config_id = c.config_id
    JOIN controller_configs cc ON c.id = cc.config_id  
    JOIN weather_site_configs wsc ON cc.id = wsc.controller_config_id
    WHERE wsc.pull_from_device = devices.name
); 