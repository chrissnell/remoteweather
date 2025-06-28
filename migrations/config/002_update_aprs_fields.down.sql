-- Revert APRS field renames back to original names
ALTER TABLE storage_configs RENAME COLUMN aprs_location_lat TO aprs_latitude;
ALTER TABLE storage_configs RENAME COLUMN aprs_location_lon TO aprs_longitude; 