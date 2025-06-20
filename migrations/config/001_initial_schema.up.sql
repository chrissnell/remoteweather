-- Main configuration table
CREATE TABLE configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL DEFAULT 'default',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Device configurations
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type TEXT,
    hostname TEXT,
    port TEXT,
    serial_device TEXT,
    baud INTEGER,
    wind_dir_correction INTEGER,
    base_snow_distance INTEGER,
    solar_latitude REAL,
    solar_longitude REAL,
    solar_altitude REAL,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Storage backend configurations
CREATE TABLE storage_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    backend_type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    
    -- InfluxDB fields
    influx_scheme TEXT,
    influx_host TEXT,
    influx_username TEXT,
    influx_password TEXT,
    influx_database TEXT,
    influx_port INTEGER,
    influx_protocol TEXT,
    
    -- TimescaleDB fields
    timescale_connection_string TEXT,
    
    -- gRPC fields
    grpc_cert TEXT,
    grpc_key TEXT,
    grpc_listen_addr TEXT,
    grpc_port INTEGER,
    grpc_pull_from_device TEXT,
    
    -- APRS fields
    aprs_callsign TEXT,
    aprs_passcode TEXT,
    aprs_server TEXT,
    aprs_latitude REAL,
    aprs_longitude REAL,
    
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, backend_type)
);

-- Controller configurations
CREATE TABLE controller_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    controller_type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    
    -- PWS Weather fields
    pws_station_id TEXT,
    pws_api_key TEXT,
    pws_upload_interval TEXT,
    pws_pull_from_device TEXT,
    pws_api_endpoint TEXT,
    
    -- Weather Underground fields
    wu_station_id TEXT,
    wu_api_key TEXT,
    wu_upload_interval TEXT,
    wu_pull_from_device TEXT,
    wu_api_endpoint TEXT,
    
    -- Aeris Weather fields
    aeris_api_client_id TEXT,
    aeris_api_client_secret TEXT,
    aeris_api_endpoint TEXT,
    aeris_location TEXT,
    
    -- REST Server fields
    rest_cert TEXT,
    rest_key TEXT,
    rest_port INTEGER,
    rest_listen_addr TEXT,
    
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, controller_type)
);

-- Weather site configuration (nested within REST server)
CREATE TABLE weather_site_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    controller_config_id INTEGER NOT NULL,
    station_name TEXT,
    pull_from_device TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device TEXT,
    snow_base_distance REAL,
    page_title TEXT,
    about_station_html TEXT,
    
    FOREIGN KEY (controller_config_id) REFERENCES controller_configs(id) ON DELETE CASCADE
);

-- Create indexes for better query performance
CREATE INDEX idx_devices_config_id ON devices(config_id);
CREATE INDEX idx_devices_name ON devices(config_id, name);
CREATE INDEX idx_storage_configs_config_id ON storage_configs(config_id);
CREATE INDEX idx_storage_configs_type ON storage_configs(config_id, backend_type);
CREATE INDEX idx_controller_configs_config_id ON controller_configs(config_id);
CREATE INDEX idx_controller_configs_type ON controller_configs(config_id, controller_type);
CREATE INDEX idx_weather_site_configs_controller_id ON weather_site_configs(controller_config_id);

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_configs_timestamp 
    AFTER UPDATE ON configs
    FOR EACH ROW
BEGIN
    UPDATE configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Insert default configuration
INSERT INTO configs (name) VALUES ('default'); 