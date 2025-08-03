-- Restore controller_configs table with all original fields including device-specific ones
-- This reverses the up migration that removed device-specific fields

-- Create the full table structure
CREATE TABLE controller_configs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    controller_type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 1,
    -- PWS Weather fields (being restored)
    pws_station_id TEXT,
    pws_api_key TEXT,
    pws_upload_interval TEXT,
    pws_pull_from_device TEXT,
    pws_api_endpoint TEXT,
    -- Weather Underground fields (being restored)
    wu_station_id TEXT,
    wu_api_key TEXT,
    wu_upload_interval TEXT,
    wu_pull_from_device TEXT,
    wu_api_endpoint TEXT,
    -- Aeris Weather fields (being restored)
    aeris_api_client_id TEXT,
    aeris_api_client_secret TEXT,
    aeris_api_endpoint TEXT,
    aeris_latitude REAL,
    aeris_longitude REAL,
    -- REST Server fields
    rest_cert TEXT,
    rest_key TEXT,
    rest_port INTEGER,
    rest_listen_addr TEXT,
    -- Management API fields
    management_cert TEXT,
    management_key TEXT,
    management_port INTEGER,
    management_listen_addr TEXT,
    management_auth_token TEXT,
    management_enable_cors BOOLEAN,
    -- APRS server field
    aprs_server TEXT,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, controller_type)
);

-- Copy data from current table (without device-specific fields)
INSERT INTO controller_configs_new (
    id, config_id, controller_type, enabled,
    pws_api_endpoint, wu_api_endpoint, aeris_api_endpoint,
    rest_cert, rest_key, rest_port, rest_listen_addr,
    management_cert, management_key, management_port, management_listen_addr,
    management_auth_token, management_enable_cors,
    aprs_server
)
SELECT 
    id, config_id, controller_type, enabled,
    pws_api_endpoint, wu_api_endpoint, aeris_api_endpoint,
    rest_cert, rest_key, rest_port, rest_listen_addr,
    management_cert, management_key, management_port, management_listen_addr,
    management_auth_token, management_enable_cors,
    aprs_server
FROM controller_configs;

-- Drop the current table
DROP TABLE controller_configs;

-- Rename the new table
ALTER TABLE controller_configs_new RENAME TO controller_configs;

-- Recreate indexes
CREATE INDEX idx_controller_configs_config_id ON controller_configs(config_id);
CREATE INDEX idx_controller_configs_type ON controller_configs(config_id, controller_type);