-- Migration 012: Add weather service fields to devices table
-- This enables each device to have its own weather service configurations

-- PWS Weather configuration
ALTER TABLE devices ADD COLUMN pws_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN pws_station_id TEXT;
ALTER TABLE devices ADD COLUMN pws_password TEXT;
ALTER TABLE devices ADD COLUMN pws_upload_interval INTEGER DEFAULT 60;

-- Weather Underground configuration
ALTER TABLE devices ADD COLUMN wu_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN wu_station_id TEXT;
ALTER TABLE devices ADD COLUMN wu_password TEXT;
ALTER TABLE devices ADD COLUMN wu_upload_interval INTEGER DEFAULT 300;

-- APRS configuration (note: aprs_enabled and aprs_callsign already exist)
ALTER TABLE devices ADD COLUMN aprs_passcode TEXT;
ALTER TABLE devices ADD COLUMN aprs_symbol_table CHAR(1) DEFAULT '/';
ALTER TABLE devices ADD COLUMN aprs_symbol_code CHAR(1) DEFAULT '_';
ALTER TABLE devices ADD COLUMN aprs_comment TEXT;

-- Aeris Weather configuration
ALTER TABLE devices ADD COLUMN aeris_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN aeris_api_client_id TEXT;
ALTER TABLE devices ADD COLUMN aeris_api_client_secret TEXT;

-- Create indexes for better query performance when filtering by enabled services
CREATE INDEX idx_devices_pws_enabled ON devices(pws_enabled) WHERE pws_enabled = TRUE;
CREATE INDEX idx_devices_wu_enabled ON devices(wu_enabled) WHERE wu_enabled = TRUE;
CREATE INDEX idx_devices_aeris_enabled ON devices(aeris_enabled) WHERE aeris_enabled = TRUE;