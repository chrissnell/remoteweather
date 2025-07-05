-- Remove individual DSN component fields from storage_configs table for TimescaleDB
ALTER TABLE storage_configs DROP COLUMN timescale_timezone;
ALTER TABLE storage_configs DROP COLUMN timescale_ssl_mode;
ALTER TABLE storage_configs DROP COLUMN timescale_password;
ALTER TABLE storage_configs DROP COLUMN timescale_user;
ALTER TABLE storage_configs DROP COLUMN timescale_database;
ALTER TABLE storage_configs DROP COLUMN timescale_port;
ALTER TABLE storage_configs DROP COLUMN timescale_host; 