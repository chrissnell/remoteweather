-- Note: SQLite doesn't support DROP COLUMN directly in older versions
-- This migration uses a table recreation approach for compatibility

-- Create temporary table without gRPC fields
CREATE TABLE controller_configs_temp AS
SELECT
    id,
    config_id,
    controller_type,
    rest_port,
    rest_listen_addr,
    management_cert,
    management_key,
    management_port,
    management_listen_addr,
    management_auth_token,
    management_enable_cors,
    aprs_server
FROM controller_configs;

-- Drop original table
DROP TABLE controller_configs;

-- Recreate table without gRPC fields
CREATE TABLE controller_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    controller_type TEXT NOT NULL,

    -- REST Server fields
    rest_port INTEGER,
    rest_listen_addr TEXT,

    -- Management API fields
    management_cert TEXT,
    management_key TEXT,
    management_port INTEGER,
    management_listen_addr TEXT,
    management_auth_token TEXT,
    management_enable_cors BOOLEAN DEFAULT FALSE,

    -- APRS server field
    aprs_server TEXT,

    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, controller_type)
);

-- Copy data back
INSERT INTO controller_configs
SELECT * FROM controller_configs_temp;

-- Drop temporary table
DROP TABLE controller_configs_temp;
