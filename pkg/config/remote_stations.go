package config

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RemoteStationModel represents a remote station in the database
// This is defined here to avoid import cycles with the database package
type RemoteStationModel struct {
	StationID      string    `gorm:"primaryKey;column:station_id"`
	StationName    string    `gorm:"column:station_name;not null;unique"`
	StationType    string    `gorm:"column:station_type;not null"`
	
	// APRS configuration
	APRSEnabled    bool      `gorm:"column:aprs_enabled;default:false"`
	APRSCallsign   string    `gorm:"column:aprs_callsign"`
	APRSPassword   string    `gorm:"column:aprs_password"`
	
	// Weather Underground configuration
	WUEnabled      bool      `gorm:"column:wu_enabled;default:false"`
	WUStationID    string    `gorm:"column:wu_station_id"`
	WUAPIKey       string    `gorm:"column:wu_api_key"`
	
	// Aeris configuration
	AerisEnabled   bool      `gorm:"column:aeris_enabled;default:false"`
	AerisClientID  string    `gorm:"column:aeris_client_id"`
	AerisClientSecret string `gorm:"column:aeris_client_secret"`
	
	// PWS configuration
	PWSEnabled     bool      `gorm:"column:pws_enabled;default:false"`
	PWSStationID   string    `gorm:"column:pws_station_id"`
	PWSPassword    string    `gorm:"column:pws_password"`
	
	RegisteredAt   time.Time `gorm:"column:registered_at;default:CURRENT_TIMESTAMP"`
	LastSeen       time.Time `gorm:"column:last_seen;default:CURRENT_TIMESTAMP"`
}

// TableName specifies the table name for RemoteStationModel
func (RemoteStationModel) TableName() string {
	return "remote_stations"
}

// RemoteStationData represents a remote station configuration
type RemoteStationData struct {
	StationID      string    `json:"station_id"`
	StationName    string    `json:"station_name"`
	StationType    string    `json:"station_type"`
	
	// Service configurations
	APRSEnabled    bool      `json:"aprs_enabled"`
	APRSCallsign   string    `json:"aprs_callsign,omitempty"`
	APRSPassword   string    `json:"aprs_password,omitempty"`
	
	WUEnabled      bool      `json:"wu_enabled"`
	WUStationID    string    `json:"wu_station_id,omitempty"`
	WUAPIKey       string    `json:"wu_api_key,omitempty"`
	
	AerisEnabled   bool      `json:"aeris_enabled"`
	AerisClientID  string    `json:"aeris_client_id,omitempty"`
	AerisClientSecret string `json:"aeris_client_secret,omitempty"`
	
	PWSEnabled     bool      `json:"pws_enabled"`
	PWSStationID   string    `json:"pws_station_id,omitempty"`
	PWSPassword    string    `json:"pws_password,omitempty"`
	
	RegisteredAt   time.Time `json:"registered_at"`
	LastSeen       time.Time `json:"last_seen"`
}

// RegisterRemoteStation registers a new remote station or updates an existing one
func (s *SQLiteProvider) RegisterRemoteStation(config *RemoteStationData) (string, error) {
	// Generate station ID if not provided
	if config.StationID == "" {
		config.StationID = uuid.New().String()
	}

	// Use INSERT OR REPLACE to handle both new and existing stations
	query := `
		INSERT OR REPLACE INTO remote_stations (
			station_id, station_name, station_type,
			aprs_enabled, aprs_callsign, aprs_password,
			wu_enabled, wu_station_id, wu_api_key,
			aeris_enabled, aeris_client_id, aeris_client_secret,
			pws_enabled, pws_station_id, pws_password,
			last_seen
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	
	_, err := s.db.Exec(query,
		config.StationID, config.StationName, config.StationType,
		config.APRSEnabled, config.APRSCallsign, config.APRSPassword,
		config.WUEnabled, config.WUStationID, config.WUAPIKey,
		config.AerisEnabled, config.AerisClientID, config.AerisClientSecret,
		config.PWSEnabled, config.PWSStationID, config.PWSPassword,
	)
	
	if err != nil {
		return "", fmt.Errorf("failed to register remote station: %w", err)
	}

	return config.StationID, nil
}

// GetRemoteStations returns all registered remote stations
func (s *SQLiteProvider) GetRemoteStations() ([]RemoteStationData, error) {
	query := `
		SELECT 
			station_id, station_name, station_type,
			aprs_enabled, aprs_callsign, aprs_password,
			wu_enabled, wu_station_id, wu_api_key,
			aeris_enabled, aeris_client_id, aeris_client_secret,
			pws_enabled, pws_station_id, pws_password,
			registered_at, last_seen
		FROM remote_stations
		ORDER BY station_name
	`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query remote stations: %w", err)
	}
	defer rows.Close()

	// Convert rows to config data
	var stations []RemoteStationData
	for rows.Next() {
		var station RemoteStationData
		err := rows.Scan(
			&station.StationID, &station.StationName, &station.StationType,
			&station.APRSEnabled, &station.APRSCallsign, &station.APRSPassword,
			&station.WUEnabled, &station.WUStationID, &station.WUAPIKey,
			&station.AerisEnabled, &station.AerisClientID, &station.AerisClientSecret,
			&station.PWSEnabled, &station.PWSStationID, &station.PWSPassword,
			&station.RegisteredAt, &station.LastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan remote station: %w", err)
		}
		stations = append(stations, station)
	}

	return stations, rows.Err()
}

// GetRemoteStation retrieves a specific remote station by ID
func (s *SQLiteProvider) GetRemoteStation(stationID string) (*RemoteStationData, error) {
	query := `
		SELECT 
			station_id, station_name, station_type,
			aprs_enabled, aprs_callsign, aprs_password,
			wu_enabled, wu_station_id, wu_api_key,
			aeris_enabled, aeris_client_id, aeris_client_secret,
			pws_enabled, pws_station_id, pws_password,
			registered_at, last_seen
		FROM remote_stations
		WHERE station_id = ?
	`

	var station RemoteStationData
	err := s.db.QueryRow(query, stationID).Scan(
		&station.StationID, &station.StationName, &station.StationType,
		&station.APRSEnabled, &station.APRSCallsign, &station.APRSPassword,
		&station.WUEnabled, &station.WUStationID, &station.WUAPIKey,
		&station.AerisEnabled, &station.AerisClientID, &station.AerisClientSecret,
		&station.PWSEnabled, &station.PWSStationID, &station.PWSPassword,
		&station.RegisteredAt, &station.LastSeen,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query remote station: %w", err)
	}
	
	return &station, nil
}

// UpdateRemoteStationLastSeen updates the last seen timestamp for a remote station
func (s *SQLiteProvider) UpdateRemoteStationLastSeen(stationID string) error {
	query := "UPDATE remote_stations SET last_seen = CURRENT_TIMESTAMP WHERE station_id = ?"
	
	result, err := s.db.Exec(query, stationID)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("remote station not found: %s", stationID)
	}
	
	return nil
}

// DeleteRemoteStation removes a remote station
func (s *SQLiteProvider) DeleteRemoteStation(stationID string) error {
	query := "DELETE FROM remote_stations WHERE station_id = ?"
	
	result, err := s.db.Exec(query, stationID)
	if err != nil {
		return fmt.Errorf("failed to delete remote station: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("remote station not found: %s", stationID)
	}
	
	return nil
}

// Implement the methods for CachedConfigProvider to pass through to underlying provider

// RegisterRemoteStation registers a remote station and invalidates cache
func (c *CachedConfigProvider) RegisterRemoteStation(config *RemoteStationData) (string, error) {
	// For SQLite provider only
	sqliteProvider, ok := c.provider.(*SQLiteProvider)
	if !ok {
		return "", fmt.Errorf("remote stations only supported with SQLite provider")
	}
	
	stationID, err := sqliteProvider.RegisterRemoteStation(config)
	if err == nil {
		c.InvalidateCache()
	}
	return stationID, err
}

// GetRemoteStations returns all remote stations (with caching)
func (c *CachedConfigProvider) GetRemoteStations() ([]RemoteStationData, error) {
	// For SQLite provider only
	sqliteProvider, ok := c.provider.(*SQLiteProvider)
	if !ok {
		return nil, fmt.Errorf("remote stations only supported with SQLite provider")
	}
	
	// TODO: Add caching for remote stations if needed
	return sqliteProvider.GetRemoteStations()
}

// GetRemoteStation retrieves a specific remote station
func (c *CachedConfigProvider) GetRemoteStation(stationID string) (*RemoteStationData, error) {
	// For SQLite provider only
	sqliteProvider, ok := c.provider.(*SQLiteProvider)
	if !ok {
		return nil, fmt.Errorf("remote stations only supported with SQLite provider")
	}
	
	return sqliteProvider.GetRemoteStation(stationID)
}

// UpdateRemoteStationLastSeen updates last seen timestamp
func (c *CachedConfigProvider) UpdateRemoteStationLastSeen(stationID string) error {
	// For SQLite provider only
	sqliteProvider, ok := c.provider.(*SQLiteProvider)
	if !ok {
		return fmt.Errorf("remote stations only supported with SQLite provider")
	}
	
	// Don't invalidate cache for last_seen updates (too frequent)
	return sqliteProvider.UpdateRemoteStationLastSeen(stationID)
}

// DeleteRemoteStation removes a remote station and invalidates cache
func (c *CachedConfigProvider) DeleteRemoteStation(stationID string) error {
	// For SQLite provider only
	sqliteProvider, ok := c.provider.(*SQLiteProvider)
	if !ok {
		return fmt.Errorf("remote stations only supported with SQLite provider")
	}
	
	err := sqliteProvider.DeleteRemoteStation(stationID)
	if err == nil {
		c.InvalidateCache()
	}
	return err
}