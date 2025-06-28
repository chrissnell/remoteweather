-- Rename APRS latitude and longitude fields to match current expectations
ALTER TABLE storage_configs RENAME COLUMN aprs_latitude TO aprs_location_lat;
ALTER TABLE storage_configs RENAME COLUMN aprs_longitude TO aprs_location_lon; 