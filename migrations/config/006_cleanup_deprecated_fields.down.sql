-- Restore deprecated fields for rollback compatibility
-- This migration restores fields that were removed in the up migration

-- Restore the TimescaleDB connection string field
ALTER TABLE storage_configs ADD COLUMN timescale_connection_string TEXT;

-- Restore the APRS passcode field
ALTER TABLE storage_configs ADD COLUMN aprs_passcode TEXT; 