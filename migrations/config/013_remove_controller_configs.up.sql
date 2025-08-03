-- Remove device-specific fields from controller_configs table
-- These fields have been moved to the devices table for multi-station support

-- SQLite doesn't support dropping columns directly, so we need to:
-- 1. Create a new table with the desired structure
-- 2. Copy data from the old table
-- 3. Drop the old table
-- 4. Rename the new table

-- Create new table without device-specific fields
CREATE TABLE controller_configs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    controller_type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 1,
    -- Global API endpoints (kept)
    pws_api_endpoint TEXT,
    wu_api_endpoint TEXT,
    aeris_api_endpoint TEXT,
    -- REST Server fields (kept)
    rest_cert TEXT,
    rest_key TEXT,
    rest_port INTEGER,
    rest_listen_addr TEXT,
    -- Management API fields (kept)
    management_cert TEXT,
    management_key TEXT,
    management_port INTEGER,
    management_listen_addr TEXT,
    management_auth_token TEXT,
    management_enable_cors BOOLEAN,
    -- APRS server field (kept)
    aprs_server TEXT,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, controller_type)
);

-- Copy data from old table (excluding device-specific fields)
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

-- Drop the old table
DROP TABLE controller_configs;

-- Rename the new table
ALTER TABLE controller_configs_new RENAME TO controller_configs;

-- Recreate indexes
CREATE INDEX idx_controller_configs_config_id ON controller_configs(config_id);
CREATE INDEX idx_controller_configs_type ON controller_configs(config_id, controller_type);