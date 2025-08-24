package database

import (
	"time"
)

// RemoteStation represents a remote weather station in the database
type RemoteStation struct {
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

// TableName specifies the table name for RemoteStation
func (RemoteStation) TableName() string {
	return "remote_stations"
}

// StorageConfig represents a storage configuration in the database
type StorageConfig struct {
	ID          int       `gorm:"primaryKey;autoIncrement;column:id"`
	ConfigID    int       `gorm:"column:config_id"`
	BackendType string    `gorm:"column:backend_type"`
	Endpoint    string    `gorm:"column:endpoint"`
	TLSEnabled  bool      `gorm:"column:tls_enabled"`
	StationID   string    `gorm:"column:station_id"`
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP"`
}

// TableName specifies the table name for StorageConfig
func (StorageConfig) TableName() string {
	return "storage_configs"
}