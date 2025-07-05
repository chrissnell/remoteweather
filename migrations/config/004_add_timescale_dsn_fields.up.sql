-- Add individual DSN component fields to storage_configs table for TimescaleDB
ALTER TABLE storage_configs ADD COLUMN timescale_host TEXT DEFAULT '';
ALTER TABLE storage_configs ADD COLUMN timescale_port INTEGER DEFAULT 5432;
ALTER TABLE storage_configs ADD COLUMN timescale_database TEXT DEFAULT '';
ALTER TABLE storage_configs ADD COLUMN timescale_user TEXT DEFAULT '';
ALTER TABLE storage_configs ADD COLUMN timescale_password TEXT DEFAULT '';
ALTER TABLE storage_configs ADD COLUMN timescale_ssl_mode TEXT DEFAULT 'prefer';
ALTER TABLE storage_configs ADD COLUMN timescale_timezone TEXT DEFAULT ''; 