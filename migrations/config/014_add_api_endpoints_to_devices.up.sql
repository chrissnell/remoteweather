-- Add API endpoint fields to devices table for per-station endpoint configuration
ALTER TABLE devices ADD COLUMN pws_api_endpoint TEXT;
ALTER TABLE devices ADD COLUMN wu_api_endpoint TEXT;
ALTER TABLE devices ADD COLUMN aeris_api_endpoint TEXT;
ALTER TABLE devices ADD COLUMN aprs_server TEXT;

-- Set default endpoints for existing devices that have services enabled
UPDATE devices 
SET pws_api_endpoint = 'https://pwsupdate.pwsweather.com/api/v1/submitwx'
WHERE pws_enabled = 1 AND pws_api_endpoint IS NULL;

UPDATE devices 
SET wu_api_endpoint = 'https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php'
WHERE wu_enabled = 1 AND wu_api_endpoint IS NULL;

UPDATE devices 
SET aeris_api_endpoint = 'https://data.api.xweather.com'
WHERE aeris_enabled = 1 AND aeris_api_endpoint IS NULL;