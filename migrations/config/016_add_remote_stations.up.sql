-- Add table for remote station registrations
CREATE TABLE IF NOT EXISTS remote_stations (
    station_id TEXT PRIMARY KEY,              -- UUID
    station_name TEXT NOT NULL UNIQUE,
    station_type TEXT NOT NULL,
    
    -- APRS configuration
    aprs_enabled BOOLEAN DEFAULT FALSE,
    aprs_callsign TEXT,
    aprs_password TEXT,
    
    -- Weather Underground configuration  
    wu_enabled BOOLEAN DEFAULT FALSE,
    wu_station_id TEXT,
    wu_api_key TEXT,
    
    -- Aeris configuration
    aeris_enabled BOOLEAN DEFAULT FALSE,
    aeris_client_id TEXT,
    aeris_client_secret TEXT,
    
    -- PWS Weather configuration
    pws_enabled BOOLEAN DEFAULT FALSE,
    pws_station_id TEXT,
    pws_password TEXT,
    
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient last_seen queries
CREATE INDEX IF NOT EXISTS idx_remote_stations_last_seen ON remote_stations(last_seen);