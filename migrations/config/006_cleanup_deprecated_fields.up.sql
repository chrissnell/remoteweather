-- Clean up deprecated fields that are no longer needed
-- This migration removes fields that have been replaced by newer implementations

-- Remove the deprecated TimescaleDB connection string field
-- (individual components are now used instead)
ALTER TABLE storage_configs DROP COLUMN timescale_connection_string;

-- Remove the unused APRS passcode field
-- (this field was never implemented and has no code references)
ALTER TABLE storage_configs DROP COLUMN aprs_passcode; 