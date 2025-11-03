# gRPC Remote Station Design

## Problem
Remote weather stations on low-power devices need to send data to a central server that handles weather services on their behalf.

## Solution

### 1. Protocol Changes

Add station registration and UUID-based authentication to the gRPC protocol:

```proto
service WeatherV1 {
    // Existing streaming RPC
    rpc SendWeatherReadings (stream WeatherReading) returns (Empty) {}
    
    // New: Remote station registration
    rpc RegisterRemoteStation (RemoteStationConfig) returns (RegistrationAck) {}
}

message RemoteStationConfig {
    string station_id = 1;        // Empty for new registration, UUID for re-registration
    string station_name = 2;
    string station_type = 3;
    
    // Service configurations - proper fields, not JSON
    bool aprs_enabled = 4;
    string aprs_callsign = 5;
    string aprs_password = 6;
    
    bool wu_enabled = 7;
    string wu_station_id = 8;
    string wu_api_key = 9;
    
    bool aeris_enabled = 10;
    string aeris_client_id = 11;
    string aeris_client_secret = 12;
    
    bool pws_enabled = 13;
    string pws_station_id = 14;
    string pws_password = 15;
}

message RegistrationAck {
    bool success = 1;
    string station_id = 2;        // UUID assigned by server
    string message = 3;
}

// Add station_id to WeatherReading
message WeatherReading {
    string station_id = 221;      // Required for remote stations
    // ... existing fields ...
}
```

### 2. Database Schema

#### Central Server

Create `migrations/config/001_add_remote_stations.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS remote_stations (
    station_id TEXT PRIMARY KEY,
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
    
    -- PWS configuration
    pws_enabled BOOLEAN DEFAULT FALSE,
    pws_station_id TEXT,
    pws_password TEXT,
    
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_remote_stations_last_seen ON remote_stations(last_seen);
```

Down migration `migrations/config/001_add_remote_stations.down.sql`:
```sql
DROP TABLE IF EXISTS remote_stations;
```

#### Remote Station

Create `migrations/config/002_add_grpcstream_fields.up.sql`:
```sql
-- Add grpcstream-specific columns to storage_configurations
CREATE TABLE storage_configurations_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    endpoint TEXT,
    tls_enabled BOOLEAN DEFAULT 1,
    station_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO storage_configurations_new (id, type, created_at, updated_at)
SELECT id, type, created_at, updated_at FROM storage_configurations;

DROP TABLE storage_configurations;
ALTER TABLE storage_configurations_new RENAME TO storage_configurations;
```

### 3. Central Server Implementation

```go
type RemoteStationRegistry struct {
    mu       sync.RWMutex
    db       *sql.DB
    stations map[string]*RemoteStation  // UUID -> station
}

type RemoteStation struct {
    StationID   string
    StationName string
    StationType string
    APRS        *APRSConfig
    WU          *WUConfig
    Aeris       *AerisConfig
    PWS         *PWSConfig
    LastSeen    time.Time
}

func (r *RemoteStationRegistry) Register(config *pb.RemoteStationConfig) (string, error) {
    stationID := config.StationId
    if stationID == "" {
        stationID = uuid.New().String()
    }
    
    _, err := r.db.Exec(`
        INSERT OR REPLACE INTO remote_stations 
        (station_id, station_name, station_type,
         aprs_enabled, aprs_callsign, aprs_password,
         wu_enabled, wu_station_id, wu_api_key,
         aeris_enabled, aeris_client_id, aeris_client_secret,
         pws_enabled, pws_station_id, pws_password,
         last_seen)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
        stationID, config.StationName, config.StationType,
        config.AprsEnabled, config.AprsCallsign, config.AprsPassword,
        config.WuEnabled, config.WuStationId, config.WuApiKey,
        config.AerisEnabled, config.AerisClientId, config.AerisClientSecret,
        config.PwsEnabled, config.PwsStationId, config.PwsPassword)
    
    if err != nil {
        return "", err
    }
    
    // Cache for fast lookups
    r.mu.Lock()
    r.stations[stationID] = r.loadStation(config)
    r.mu.Unlock()
    
    return stationID, nil
}

func (r *RemoteStationRegistry) ProcessReading(reading *pb.WeatherReading) {
    if reading.StationId == "" {
        return
    }
    
    r.mu.RLock()
    station, exists := r.stations[reading.StationId]
    r.mu.RUnlock()
    
    if !exists {
        return
    }
    
    // Update last seen
    r.db.Exec("UPDATE remote_stations SET last_seen = CURRENT_TIMESTAMP WHERE station_id = ?", 
              reading.StationId)
    
    // Forward to enabled services
    internalReading := convertReading(reading)
    if station.APRS != nil {
        go r.aprsClient.Send(internalReading, station.APRS)
    }
    if station.WU != nil {
        go r.wuClient.Send(internalReading, station.WU)
    }
    // etc.
}
```

### 4. Remote Station Implementation

```go
type GRPCStreamStorage struct {
    db        *sql.DB
    stationID string
    client    pb.WeatherV1Client
    stream    pb.WeatherV1_SendWeatherReadingsClient
}

func (s *GRPCStreamStorage) Initialize() error {
    // Load saved UUID
    var stationID sql.NullString
    s.db.QueryRow("SELECT station_id FROM storage_configurations WHERE type = 'grpcstream'").Scan(&stationID)
    
    if stationID.Valid {
        s.stationID = stationID.String
    }
    
    // Build registration from device config
    config := s.buildRegistrationConfig()
    config.StationId = s.stationID
    
    // Register with server
    resp, err := s.client.RegisterRemoteStation(context.Background(), config)
    if err != nil {
        return err
    }
    
    // Save UUID if new
    if s.stationID != resp.StationId {
        s.stationID = resp.StationId
        s.db.Exec("UPDATE storage_configurations SET station_id = ? WHERE type = 'grpcstream'", 
                  s.stationID)
    }
    
    // Start streaming
    s.stream, err = s.client.SendWeatherReadings(context.Background())
    return err
}

func (s *GRPCStreamStorage) StoreReading(reading types.Reading) error {
    pbReading := convertToProto(reading)
    pbReading.StationId = s.stationID
    return s.stream.Send(pbReading)
}
```

### 5. Management UI

Add `/api/remote-stations` endpoint:

```go
func (s *Server) GetRemoteStations(w http.ResponseWriter, r *http.Request) {
    rows, _ := s.db.Query(`
        SELECT station_id, station_name, station_type, 
               aprs_enabled, wu_enabled, aeris_enabled, pws_enabled,
               registered_at, last_seen
        FROM remote_stations
        ORDER BY station_name
    `)
    
    // Return JSON array of stations with status calculation
}
```

UI displays:
- Station Name
- Type
- Enabled Services (checkmarks)
- Last Seen (with color coding: green < 5min, yellow < 1hr, red otherwise)

### 6. Implementation Steps

1. Create and apply database migrations
2. Update protobuf definitions and regenerate
3. Implement RemoteStationRegistry on central server
4. Update GRPCStreamStorage to handle registration
5. Add remote stations API endpoint
6. Add UI tab for remote stations

## Key Design Decisions

- **UUID Authentication**: Simple, stateless, works like API keys
- **Direct Service Forwarding**: No complex controller spawning
- **SQLite with Proper Schema**: No JSON blobs, proper columns
- **In-Memory Cache**: Fast lookups without hitting DB for every reading
- **Minimal Protocol Changes**: Just add registration RPC and station_id field

## What This Intentionally Doesn't Do

- No dynamic config updates (requires re-registration)
- No fallback mechanisms (remote stations are remote-only)
- No complex state synchronization