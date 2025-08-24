-- SQLite doesn't support DROP COLUMN directly, need to recreate table
-- Create new table without station_id column
CREATE TABLE storage_configs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    backend_type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    
    -- TimescaleDB fields
    timescale_host TEXT DEFAULT '',
    timescale_port INTEGER DEFAULT 5432,
    timescale_database TEXT DEFAULT '',
    timescale_user TEXT DEFAULT '',
    timescale_password TEXT DEFAULT '',
    timescale_ssl_mode TEXT DEFAULT 'prefer',
    timescale_timezone TEXT DEFAULT '',
    
    -- gRPC fields
    grpc_cert TEXT,
    grpc_key TEXT,
    grpc_listen_addr TEXT,
    grpc_port INTEGER,
    grpc_pull_from_device TEXT,
    
    -- APRS fields
    aprs_callsign TEXT,
    aprs_server TEXT,
    aprs_location_lat REAL,
    aprs_location_lon REAL,

    -- Health status fields
    health_last_check DATETIME,
    health_status TEXT DEFAULT 'unknown',
    health_message TEXT,
    health_error TEXT,
    
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, backend_type)
);

-- Copy data from existing table (excluding station_id)
INSERT INTO storage_configs_new 
SELECT id, config_id, backend_type, enabled,
       timescale_host, timescale_port, timescale_database, 
       timescale_user, timescale_password, timescale_ssl_mode, timescale_timezone,
       grpc_cert, grpc_key, grpc_listen_addr, grpc_port, grpc_pull_from_device,
       aprs_callsign, aprs_server, aprs_location_lat, aprs_location_lon,
       health_last_check, health_status, health_message, health_error
FROM storage_configs;

-- Replace old table with new
DROP TABLE storage_configs;
ALTER TABLE storage_configs_new RENAME TO storage_configs;

-- Recreate indexes
CREATE INDEX idx_storage_configs_config_id ON storage_configs(config_id);
CREATE INDEX idx_storage_configs_type ON storage_configs(config_id, backend_type);
CREATE INDEX idx_storage_configs_health_status ON storage_configs(health_status);
CREATE INDEX idx_storage_configs_health_last_check ON storage_configs(health_last_check);