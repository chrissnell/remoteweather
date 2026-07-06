-- Migration 027: Configurable send/upload intervals
-- APRS gains a per-device upload interval (seconds); Aeris gains an optional
-- forecast refresh override (seconds). Both default to the values in use today:
-- APRS 300s (5 min), and Aeris 0 which means "derive from the forecast period"
-- (the existing 4x-per-period behavior).

ALTER TABLE devices ADD COLUMN aprs_upload_interval INTEGER DEFAULT 300;
ALTER TABLE devices ADD COLUMN aeris_refresh_interval INTEGER DEFAULT 0;
