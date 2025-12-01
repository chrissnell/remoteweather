package config

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteProvider implements ConfigProvider for SQLite database configuration
type SQLiteProvider struct {
	db     *sql.DB
	dbPath string
}

// NewSQLiteProvider creates a new SQLite configuration provider
func NewSQLiteProvider(dbPath string) (*SQLiteProvider, error) {
	// Open with connection string that includes parameters
	// _busy_timeout: Wait up to 10 seconds when database is locked
	// _journal_mode=WAL: Write-Ahead Logging for better concurrency
	// _synchronous=NORMAL: Good balance of safety and performance
	connStr := fmt.Sprintf("%s?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Set connection pool settings for better concurrency
	// Allow multiple readers but serialize writes
	db.SetMaxOpenConns(10)   // Allow multiple readers
	db.SetMaxIdleConns(5)    // Keep some connections ready
	db.SetConnMaxLifetime(0) // Don't close connections due to age
	db.SetConnMaxIdleTime(0) // Don't close idle connections

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	provider := &SQLiteProvider{
		db:     db,
		dbPath: dbPath,
	}

	// Check if the database needs to be initialized (no tables exist)
	if err := provider.initializeSchemaIfNeeded(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Execute additional PRAGMA statements for better performance and reliability
	pragmas := []string{
		"PRAGMA temp_store = MEMORY",   // Use memory for temporary tables
		"PRAGMA mmap_size = 268435456", // Use memory-mapped I/O (256MB)
		"PRAGMA cache_size = -64000",   // Use 64MB for cache
		"PRAGMA foreign_keys = ON",     // Enable foreign key constraints
		"PRAGMA optimize",              // Optimize database on open
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			// Log warning but don't fail - some pragmas might not be critical
			// You may want to add proper logging here
		}
	}

	return provider, nil
}

// initializeSchemaIfNeeded checks if the database is empty and initializes the schema
func (s *SQLiteProvider) initializeSchemaIfNeeded() error {
	// Check if the configs table exists
	var tableName string
	err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='configs'").Scan(&tableName)
	if err == sql.ErrNoRows {
		// Database is empty, initialize schema
		return s.initializeSchema()
	} else if err != nil {
		return fmt.Errorf("failed to check for existing tables: %w", err)
	}

	// Tables exist, no initialization needed
	return nil
}

// initializeSchema creates all necessary tables and initial data
func (s *SQLiteProvider) initializeSchema() error {
	// Create all tables with the complete schema from migrations
	schema := `
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
    enabled BOOLEAN DEFAULT TRUE,
    hostname TEXT,
    port TEXT,
    serial_device TEXT,
    baud INTEGER,
    wind_dir_correction INTEGER,
    base_snow_distance INTEGER,
    website_id INTEGER,
    latitude REAL,
    longitude REAL,
    altitude REAL,
    aprs_enabled BOOLEAN DEFAULT FALSE,
    aprs_callsign TEXT,
    tls_cert_file TEXT,
    tls_key_file TEXT,
    path TEXT DEFAULT '',
    -- PWS Weather fields
    pws_enabled BOOLEAN DEFAULT FALSE,
    pws_station_id TEXT,
    pws_password TEXT,
    pws_upload_interval INTEGER DEFAULT 60,
    pws_api_endpoint TEXT,
    -- Weather Underground fields
    wu_enabled BOOLEAN DEFAULT FALSE,
    wu_station_id TEXT,
    wu_password TEXT,
    wu_upload_interval INTEGER DEFAULT 300,
    wu_api_endpoint TEXT,
    -- APRS additional fields
    aprs_passcode TEXT,
    aprs_symbol_table CHAR(1) DEFAULT '/',
    aprs_symbol_code CHAR(1) DEFAULT '_',
    aprs_comment TEXT,
    aprs_server TEXT,
    -- Aeris Weather fields
    aeris_enabled BOOLEAN DEFAULT FALSE,
    aeris_api_client_id TEXT,
    aeris_api_client_secret TEXT,
    aeris_api_endpoint TEXT,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, name)
);

-- Storage backend configurations
CREATE TABLE storage_configs (
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
    station_id TEXT,  -- UUID for remote gRPC stations
    
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
		aeris_latitude REAL,
		aeris_longitude REAL,
    
    -- REST Server fields
    rest_port INTEGER,
    rest_listen_addr TEXT,

    -- gRPC Server fields (added in migration 020)
    grpc_port INTEGER DEFAULT 50051,
    grpc_listen_addr TEXT DEFAULT '0.0.0.0',
    grpc_cert TEXT,
    grpc_key TEXT,

    -- Management API fields
    management_cert TEXT,
    management_key TEXT,
    management_port INTEGER,
    management_listen_addr TEXT,
    management_auth_token TEXT,
    management_enable_cors BOOLEAN DEFAULT FALSE,

    -- APRS server field (added in migration 005)
    aprs_server TEXT,
    
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, controller_type)
);


-- Weather websites table (from migration 003)
CREATE TABLE weather_websites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    device_id INTEGER,
    hostname TEXT,
    page_title TEXT,
    about_station_html TEXT,
    snow_enabled BOOLEAN DEFAULT FALSE,
    snow_device_name TEXT,
    air_quality_enabled BOOLEAN DEFAULT FALSE,
    air_quality_device_name TEXT,
    tls_cert_path TEXT,
    tls_key_path TEXT,
    is_portal BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES configs(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL,
    UNIQUE(config_id, name)
);


-- Create indexes for better query performance
CREATE INDEX idx_devices_config_id ON devices(config_id);
CREATE INDEX idx_devices_name ON devices(config_id, name);
CREATE INDEX idx_devices_pws_enabled ON devices(pws_enabled) WHERE pws_enabled = TRUE;
CREATE INDEX idx_devices_wu_enabled ON devices(wu_enabled) WHERE wu_enabled = TRUE;
CREATE INDEX idx_devices_aeris_enabled ON devices(aeris_enabled) WHERE aeris_enabled = TRUE;
CREATE INDEX idx_storage_configs_config_id ON storage_configs(config_id);
CREATE INDEX idx_storage_configs_type ON storage_configs(config_id, backend_type);
CREATE INDEX idx_storage_configs_health_status ON storage_configs(health_status);
CREATE INDEX idx_storage_configs_health_last_check ON storage_configs(health_last_check);
CREATE INDEX idx_controller_configs_config_id ON controller_configs(config_id);
CREATE INDEX idx_controller_configs_type ON controller_configs(config_id, controller_type);
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id);

-- Remote stations table for gRPC remote station registrations
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
    
    -- PWS Weather configuration
    pws_enabled BOOLEAN DEFAULT FALSE,
    pws_station_id TEXT,
    pws_password TEXT,
    
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_remote_stations_last_seen ON remote_stations(last_seen);

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_configs_timestamp 
    AFTER UPDATE ON configs
    FOR EACH ROW
BEGIN
    UPDATE configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Insert default configuration
INSERT INTO configs (name) VALUES ('default');
`

	// Execute the schema creation
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	return nil
}

// LoadConfig loads the complete configuration from SQLite database
func (s *SQLiteProvider) LoadConfig() (*ConfigData, error) {
	config := &ConfigData{}

	// Load devices
	devices, err := s.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to load devices: %w", err)
	}
	config.Devices = devices

	// Load storage
	storage, err := s.GetStorageConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load storage config: %w", err)
	}
	config.Storage = *storage

	// Load controllers
	controllers, err := s.GetControllers()
	if err != nil {
		return nil, fmt.Errorf("failed to load controllers: %w", err)
	}
	config.Controllers = controllers

	return config, nil
}

// GetDevices returns device configurations from the database
func (s *SQLiteProvider) GetDevices() ([]DeviceData, error) {
	query := `
		SELECT id, name, type, enabled, hostname, port, serial_device, baud, 
		       wind_dir_correction, base_snow_distance, website_id,
		       latitude, longitude, altitude, aprs_enabled, aprs_callsign,
		       tls_cert_file, tls_key_file, path,
		       pws_enabled, pws_station_id, pws_password, pws_upload_interval, pws_api_endpoint,
		       wu_enabled, wu_station_id, wu_password, wu_upload_interval, wu_api_endpoint,
		       aprs_passcode, aprs_symbol_table, aprs_symbol_code, aprs_comment, aprs_server,
		       aeris_enabled, aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint
		FROM devices 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default')
		ORDER BY name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	var devices []DeviceData
	for rows.Next() {
		var device DeviceData
		var hostname, port, serialDevice, aprsCallsign sql.NullString
		var tlsCertFile, tlsKeyFile, path sql.NullString
		var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
		var latitude, longitude, altitude sql.NullFloat64
		var enabled sql.NullBool
		var aprsEnabled sql.NullBool

		// PWS Weather fields
		var pwsEnabled sql.NullBool
		var pwsStationID, pwsPassword, pwsAPIEndpoint sql.NullString
		var pwsUploadInterval sql.NullInt64

		// Weather Underground fields
		var wuEnabled sql.NullBool
		var wuStationID, wuPassword, wuAPIEndpoint sql.NullString
		var wuUploadInterval sql.NullInt64

		// APRS additional fields
		var aprsPasscode, aprsSymbolTable, aprsSymbolCode, aprsComment, aprsServer sql.NullString

		// Aeris Weather fields
		var aerisEnabled sql.NullBool
		var aerisAPIClientID, aerisAPIClientSecret, aerisAPIEndpoint sql.NullString

		err := rows.Scan(
			&device.ID, &device.Name, &device.Type, &enabled, &hostname, &port,
			&serialDevice, &baud, &windDirCorrection,
			&baseSnowDistance, &websiteID, &latitude, &longitude, &altitude,
			&aprsEnabled, &aprsCallsign, &tlsCertFile, &tlsKeyFile, &path,
			&pwsEnabled, &pwsStationID, &pwsPassword, &pwsUploadInterval, &pwsAPIEndpoint,
			&wuEnabled, &wuStationID, &wuPassword, &wuUploadInterval, &wuAPIEndpoint,
			&aprsPasscode, &aprsSymbolTable, &aprsSymbolCode, &aprsComment, &aprsServer,
			&aerisEnabled, &aerisAPIClientID, &aerisAPIClientSecret, &aerisAPIEndpoint,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}

		// Convert nullable string fields to empty strings if NULL
		if hostname.Valid {
			device.Hostname = hostname.String
		}
		if port.Valid {
			device.Port = port.String
		}
		if serialDevice.Valid {
			device.SerialDevice = serialDevice.String
		}

		// Convert nullable integer fields to zero if NULL
		if baud.Valid {
			device.Baud = int(baud.Int64)
		}
		if windDirCorrection.Valid {
			device.WindDirCorrection = int16(windDirCorrection.Int64)
		}
		if baseSnowDistance.Valid {
			device.BaseSnowDistance = int16(baseSnowDistance.Int64)
		}

		// Set website ID if present
		if websiteID.Valid {
			websiteIDInt := int(websiteID.Int64)
			device.WebsiteID = &websiteIDInt
		}

		// Set location data if present
		if latitude.Valid {
			device.Latitude = latitude.Float64
		}
		if longitude.Valid {
			device.Longitude = longitude.Float64
		}
		if altitude.Valid {
			device.Altitude = altitude.Float64
		}

		// Set device enabled
		if enabled.Valid {
			device.Enabled = enabled.Bool
		}

		// Set APRS data
		device.APRSEnabled = aprsEnabled.Bool
		device.APRSCallsign = aprsCallsign.String

		// Set TLS and path data
		if tlsCertFile.Valid {
			device.TLSCertPath = tlsCertFile.String
		}
		if tlsKeyFile.Valid {
			device.TLSKeyPath = tlsKeyFile.String
		}
		if path.Valid {
			device.Path = path.String
		}

		// Set PWS Weather fields
		device.PWSEnabled = pwsEnabled.Bool
		if pwsStationID.Valid {
			device.PWSStationID = pwsStationID.String
		}
		if pwsPassword.Valid {
			device.PWSPassword = pwsPassword.String
		}
		if pwsUploadInterval.Valid {
			device.PWSUploadInterval = int(pwsUploadInterval.Int64)
		}
		if pwsAPIEndpoint.Valid {
			device.PWSAPIEndpoint = pwsAPIEndpoint.String
		}

		// Set Weather Underground fields
		device.WUEnabled = wuEnabled.Bool
		if wuStationID.Valid {
			device.WUStationID = wuStationID.String
		}
		if wuPassword.Valid {
			device.WUPassword = wuPassword.String
		}
		if wuUploadInterval.Valid {
			device.WUUploadInterval = int(wuUploadInterval.Int64)
		}
		if wuAPIEndpoint.Valid {
			device.WUAPIEndpoint = wuAPIEndpoint.String
		}

		// Set APRS additional fields
		if aprsPasscode.Valid {
			device.APRSPasscode = aprsPasscode.String
		}
		if aprsSymbolTable.Valid {
			device.APRSSymbolTable = aprsSymbolTable.String
		}
		if aprsSymbolCode.Valid {
			device.APRSSymbolCode = aprsSymbolCode.String
		}
		if aprsComment.Valid {
			device.APRSComment = aprsComment.String
		}
		if aprsServer.Valid {
			device.APRSServer = aprsServer.String
		}

		// Set Aeris Weather fields
		device.AerisEnabled = aerisEnabled.Bool
		if aerisAPIClientID.Valid {
			device.AerisAPIClientID = aerisAPIClientID.String
		}
		if aerisAPIClientSecret.Valid {
			device.AerisAPIClientSecret = aerisAPIClientSecret.String
		}
		if aerisAPIEndpoint.Valid {
			device.AerisAPIEndpoint = aerisAPIEndpoint.String
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// GetStorageConfig returns storage configuration from the database
func (s *SQLiteProvider) GetStorageConfig() (*StorageData, error) {
	query := `
		SELECT backend_type, enabled,
		       -- TimescaleDB fields
		       timescale_host, timescale_port, timescale_database, timescale_user,
		       timescale_password, timescale_ssl_mode, timescale_timezone,
		       -- gRPC fields
		       grpc_cert, grpc_key, grpc_listen_addr, grpc_port, grpc_pull_from_device,
		       -- APRS fields
		       aprs_callsign, aprs_server, aprs_location_lat, aprs_location_lon,
		       -- Health fields
		       health_last_check, health_status, health_message, health_error
		FROM storage_configs 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default') AND enabled = 1
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query storage configs: %w", err)
	}
	defer rows.Close()

	storage := &StorageData{}

	for rows.Next() {
		var backendType string
		var enabled bool
		var timescaleHost, timescaleDatabase, timescaleUser, timescalePassword, timescaleSSLMode, timescaleTimezone sql.NullString
		var timescalePort sql.NullInt64
		var grpcCert, grpcKey, grpcListenAddr, grpcPullFromDevice sql.NullString
		var grpcPort sql.NullInt64
		// Note: APRS fields removed - now handled by separate APRS tables
		var aprsCallsign, aprsServer sql.NullString
		var aprsLat, aprsLon sql.NullFloat64
		// Health fields
		var healthLastCheck sql.NullTime
		var healthStatus, healthMessage, healthError sql.NullString

		err := rows.Scan(
			&backendType, &enabled,
			&timescaleHost, &timescalePort, &timescaleDatabase, &timescaleUser,
			&timescalePassword, &timescaleSSLMode, &timescaleTimezone,
			&grpcCert, &grpcKey, &grpcListenAddr, &grpcPort, &grpcPullFromDevice,
			&aprsCallsign, &aprsServer, &aprsLat, &aprsLon,
			&healthLastCheck, &healthStatus, &healthMessage, &healthError,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan storage config row: %w", err)
		}

		switch backendType {
		case "timescaledb":
			if timescaleHost.Valid {
				storage.TimescaleDB = &TimescaleDBData{
					Host:     timescaleHost.String,
					Port:     int(timescalePort.Int64),
					Database: timescaleDatabase.String,
					User:     timescaleUser.String,
					Password: timescalePassword.String,
					SSLMode:  timescaleSSLMode.String,
					Timezone: timescaleTimezone.String,
				}
			}
		case "grpc":
			if grpcPort.Valid {
				storage.GRPC = &GRPCData{
					Cert:           grpcCert.String,
					Key:            grpcKey.String,
					ListenAddr:     grpcListenAddr.String,
					Port:           int(grpcPort.Int64),
					PullFromDevice: grpcPullFromDevice.String,
				}
			}
		}
	}

	return storage, nil
}

// GetControllers returns controller configurations from the database
func (s *SQLiteProvider) GetControllers() ([]ControllerData, error) {
	query := `
		SELECT cc.controller_type, cc.enabled,
		       -- Global API endpoints only (device-specific fields removed)
		       cc.pws_api_endpoint,
		       cc.wu_api_endpoint,
		       cc.aeris_api_endpoint,
		       -- REST Server fields
		       cc.rest_port, cc.rest_listen_addr,
		       -- gRPC Server fields
		       cc.grpc_port, cc.grpc_listen_addr, cc.grpc_cert, cc.grpc_key,
		       -- Management API fields
		       cc.management_cert, cc.management_key, cc.management_port, cc.management_listen_addr,
		       cc.management_auth_token, cc.management_enable_cors,
		       -- APRS fields
		       cc.aprs_server
		FROM controller_configs cc
		WHERE cc.config_id = (SELECT id FROM configs WHERE name = 'default') AND cc.enabled = 1
		ORDER BY cc.controller_type
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query controller configs: %w", err)
	}
	defer rows.Close()

	var controllers []ControllerData

	for rows.Next() {
		var controllerType string
		var enabled bool
		var pwsAPIEndpoint, wuAPIEndpoint, aerisAPIEndpoint sql.NullString
		var restListenAddr sql.NullString
		var restPort sql.NullInt64
		var grpcListenAddr, grpcCert, grpcKey sql.NullString
		var grpcPort sql.NullInt64
		var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
		var mgmtPort sql.NullInt64
		var mgmtEnableCORS sql.NullBool
		var aprsServer sql.NullString

		err := rows.Scan(
			&controllerType, &enabled,
			&pwsAPIEndpoint,
			&wuAPIEndpoint,
			&aerisAPIEndpoint,
			&restPort, &restListenAddr,
			&grpcPort, &grpcListenAddr, &grpcCert, &grpcKey,
			&mgmtCert, &mgmtKey, &mgmtPort, &mgmtListenAddr, &mgmtAuthToken, &mgmtEnableCORS,
			&aprsServer,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan controller config row: %w", err)
		}

		controller := ControllerData{
			Type: controllerType,
		}

		switch controllerType {
		case "pwsweather":
			// Create controller with just API endpoint
			// Device-specific settings are now in devices table
			controller.PWSWeather = &PWSWeatherData{
				APIEndpoint: pwsAPIEndpoint.String,
			}
		case "weatherunderground":
			// Create controller with just API endpoint
			// Device-specific settings are now in devices table
			controller.WeatherUnderground = &WeatherUndergroundData{
				APIEndpoint: wuAPIEndpoint.String,
			}
		case "aerisweather":
			// Create controller with just API endpoint
			// Device-specific settings are now in devices table
			controller.AerisWeather = &AerisWeatherData{
				APIEndpoint: aerisAPIEndpoint.String,
			}
		case "rest":
			if restPort.Valid || restListenAddr.Valid {
				controller.RESTServer = &RESTServerData{
					HTTPPort:          int(restPort.Int64),
					DefaultListenAddr: restListenAddr.String,
					GRPCPort:          int(grpcPort.Int64),
					GRPCListenAddr:    grpcListenAddr.String,
					GRPCCertPath:      grpcCert.String,
					GRPCKeyPath:       grpcKey.String,
				}
			}
		case "management":
			if mgmtPort.Valid {
				controller.ManagementAPI = &ManagementAPIData{
					Cert:       mgmtCert.String,
					Key:        mgmtKey.String,
					Port:       int(mgmtPort.Int64),
					ListenAddr: mgmtListenAddr.String,
					AuthToken:  mgmtAuthToken.String,
				}
			}
		case "aprs":
			if aprsServer.Valid {
				controller.APRS = &APRSData{
					Server: aprsServer.String,
				}
			}
		}

		controllers = append(controllers, controller)
	}

	return controllers, nil
}

// IsReadOnly returns false since SQLite configuration can be modified
func (s *SQLiteProvider) IsReadOnly() bool {
	return false
}

// Close closes the database connection
func (s *SQLiteProvider) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection
func (s *SQLiteProvider) GetDB() *sql.DB {
	return s.db
}

// Write methods for configuration management

// SaveConfig saves complete configuration to the database
func (s *SQLiteProvider) SaveConfig(configData *ConfigData) error {
	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert or update config record
	configID, err := s.insertConfig(tx, "default")
	if err != nil {
		return fmt.Errorf("failed to insert config: %w", err)
	}

	// Clear existing data
	if err := s.clearExistingConfig(tx, configID); err != nil {
		return fmt.Errorf("failed to clear existing config: %w", err)
	}

	// Insert devices
	for _, device := range configData.Devices {
		if err := s.insertDevice(tx, configID, &device); err != nil {
			return fmt.Errorf("failed to insert device %s: %w", device.Name, err)
		}
	}

	// Insert storage configuration
	if err := s.insertStorageConfigs(tx, configID, &configData.Storage); err != nil {
		return fmt.Errorf("failed to insert storage configs: %w", err)
	}

	// Insert controllers
	for _, controller := range configData.Controllers {
		if err := s.insertController(tx, configID, &controller); err != nil {
			return fmt.Errorf("failed to insert controller %s: %w", controller.Type, err)
		}
	}

	// Commit transaction
	return tx.Commit()
}

func (s *SQLiteProvider) insertConfig(tx *sql.Tx, name string) (int64, error) {
	query := `INSERT OR REPLACE INTO configs (name, created_at, updated_at) VALUES (?, datetime('now'), datetime('now'))`
	result, err := tx.Exec(query, name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *SQLiteProvider) clearExistingConfig(tx *sql.Tx, configID int64) error {
	queries := []string{
		"DELETE FROM devices WHERE config_id = ?",
		"DELETE FROM storage_configs WHERE config_id = ?",
		"DELETE FROM controller_configs WHERE config_id = ?",
		"DELETE FROM weather_websites WHERE config_id = ?",
	}

	for _, query := range queries {
		if _, err := tx.Exec(query, configID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteProvider) insertDevice(tx *sql.Tx, configID int64, device *DeviceData) error {
	query := `
		INSERT INTO devices (
			config_id, name, type, enabled, hostname, port, serial_device,
			baud, wind_dir_correction, base_snow_distance, website_id,
			latitude, longitude, altitude, aprs_enabled, aprs_callsign,
			tls_cert_file, tls_key_file, path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var websiteID sql.NullInt64
	if device.WebsiteID != nil {
		websiteID = sql.NullInt64{Int64: int64(*device.WebsiteID), Valid: true}
	}

	_, err := tx.Exec(query,
		configID, device.Name, device.Type, device.Enabled, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		websiteID, device.Latitude, device.Longitude, device.Altitude,
		device.APRSEnabled, device.APRSCallsign,
		nullString(device.TLSCertPath), nullString(device.TLSKeyPath), nullString(device.Path),
	)
	return err
}

func (s *SQLiteProvider) insertStorageConfigs(tx *sql.Tx, configID int64, storage *StorageData) error {
	if storage.TimescaleDB != nil {
		if err := s.insertTimescaleDBConfig(tx, configID, storage.TimescaleDB); err != nil {
			return err
		}
	}

	if storage.GRPC != nil {
		if err := s.insertGRPCConfig(tx, configID, storage.GRPC); err != nil {
			return err
		}
	}

	// APRS configs now handled separately via APRS management methods

	return nil
}

func (s *SQLiteProvider) insertTimescaleDBConfig(tx *sql.Tx, configID int64, timescale *TimescaleDBData) error {
	query := `
		INSERT INTO storage_configs (
			config_id, backend_type, enabled, 
			timescale_host, timescale_port, timescale_database, timescale_user, 
			timescale_password, timescale_ssl_mode, timescale_timezone
		) VALUES (?, 'timescaledb', 1, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, configID,
		timescale.Host, timescale.Port, timescale.Database, timescale.User,
		timescale.Password, timescale.SSLMode, timescale.Timezone)
	return err
}

func (s *SQLiteProvider) insertGRPCConfig(tx *sql.Tx, configID int64, grpc *GRPCData) error {
	query := `
		INSERT INTO storage_configs (
			config_id, backend_type, enabled,
			grpc_cert, grpc_key, grpc_listen_addr, grpc_port, grpc_pull_from_device
		) VALUES (?, 'grpc', 1, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, configID,
		grpc.Cert, grpc.Key, grpc.ListenAddr, grpc.Port, grpc.PullFromDevice,
	)
	return err
}

// insertAPRSConfig removed - APRS is now handled by separate APRS management methods

func (s *SQLiteProvider) insertController(tx *sql.Tx, configID int64, controller *ControllerData) error {
	// Insert controller record
	// Note: Device-specific fields (station IDs, API keys, etc.) were moved to devices table in migration 013
	// Only global API endpoints and server configuration fields remain in controller_configs
	query := `
		INSERT INTO controller_configs (
			config_id, controller_type, enabled,
			pws_api_endpoint,
			wu_api_endpoint,
			aeris_api_endpoint,
			rest_port, rest_listen_addr,
			grpc_port, grpc_listen_addr, grpc_cert, grpc_key,
			management_cert, management_key, management_port, management_listen_addr,
			management_auth_token, management_enable_cors, aprs_server
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var pwsAPIEndpoint sql.NullString
	var wuAPIEndpoint sql.NullString
	var aerisAPIEndpoint sql.NullString
	var restListenAddr sql.NullString
	var restPort sql.NullInt64
	var grpcListenAddr, grpcCert, grpcKey sql.NullString
	var grpcPort sql.NullInt64
	var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
	var mgmtPort sql.NullInt64
	var mgmtEnableCORS sql.NullBool
	var aprsServer sql.NullString

	if controller.PWSWeather != nil {
		pwsAPIEndpoint = sql.NullString{String: controller.PWSWeather.APIEndpoint, Valid: controller.PWSWeather.APIEndpoint != ""}
	}

	if controller.WeatherUnderground != nil {
		wuAPIEndpoint = sql.NullString{String: controller.WeatherUnderground.APIEndpoint, Valid: controller.WeatherUnderground.APIEndpoint != ""}
	}

	if controller.AerisWeather != nil {
		aerisAPIEndpoint = sql.NullString{String: controller.AerisWeather.APIEndpoint, Valid: controller.AerisWeather.APIEndpoint != ""}
	}

	if controller.RESTServer != nil {
		restPort = sql.NullInt64{Int64: int64(controller.RESTServer.HTTPPort), Valid: controller.RESTServer.HTTPPort != 0}
		restListenAddr = sql.NullString{String: controller.RESTServer.DefaultListenAddr, Valid: controller.RESTServer.DefaultListenAddr != ""}
		grpcPort = sql.NullInt64{Int64: int64(controller.RESTServer.GRPCPort), Valid: controller.RESTServer.GRPCPort != 0}
		grpcListenAddr = sql.NullString{String: controller.RESTServer.GRPCListenAddr, Valid: controller.RESTServer.GRPCListenAddr != ""}
		grpcCert = sql.NullString{String: controller.RESTServer.GRPCCertPath, Valid: controller.RESTServer.GRPCCertPath != ""}
		grpcKey = sql.NullString{String: controller.RESTServer.GRPCKeyPath, Valid: controller.RESTServer.GRPCKeyPath != ""}
	}

	if controller.ManagementAPI != nil {
		mgmtCert = sql.NullString{String: controller.ManagementAPI.Cert, Valid: controller.ManagementAPI.Cert != ""}
		mgmtKey = sql.NullString{String: controller.ManagementAPI.Key, Valid: controller.ManagementAPI.Key != ""}
		mgmtPort = sql.NullInt64{Int64: int64(controller.ManagementAPI.Port), Valid: controller.ManagementAPI.Port != 0}
		mgmtListenAddr = sql.NullString{String: controller.ManagementAPI.ListenAddr, Valid: controller.ManagementAPI.ListenAddr != ""}
		mgmtAuthToken = sql.NullString{String: controller.ManagementAPI.AuthToken, Valid: controller.ManagementAPI.AuthToken != ""}
		mgmtEnableCORS = sql.NullBool{Bool: true, Valid: true} // CORS is always enabled
	}

	if controller.APRS != nil {
		aprsServer = sql.NullString{String: controller.APRS.Server, Valid: controller.APRS.Server != ""}
	}

	_, err := tx.Exec(query, configID, controller.Type,
		pwsAPIEndpoint,
		wuAPIEndpoint,
		aerisAPIEndpoint,
		restPort, restListenAddr,
		grpcPort, grpcListenAddr, grpcCert, grpcKey,
		mgmtCert, mgmtKey, mgmtPort, mgmtListenAddr, mgmtAuthToken, mgmtEnableCORS, aprsServer,
	)
	if err != nil {
		return err
	}

	// Note: Weather site config is now handled separately via weather_websites table
	return nil
}

// Individual device management methods

// GetDevice retrieves a specific device by name
func (s *SQLiteProvider) GetDevice(name string) (*DeviceData, error) {
	query := `
		SELECT d.id, d.name, d.type, d.enabled, d.hostname, d.port, d.serial_device, d.baud,
		       d.wind_dir_correction, d.base_snow_distance, d.website_id,
		       d.latitude, d.longitude, d.altitude, d.aprs_enabled, d.aprs_callsign,
		       d.tls_cert_file, d.tls_key_file, d.path,
		       d.pws_enabled, d.pws_station_id, d.pws_password, d.pws_upload_interval,
		       d.wu_enabled, d.wu_station_id, d.wu_password, d.wu_upload_interval,
		       d.aprs_passcode, d.aprs_symbol_table, d.aprs_symbol_code, d.aprs_comment,
		       d.aeris_enabled, d.aeris_api_client_id, d.aeris_api_client_secret
		FROM devices d
		JOIN configs c ON d.config_id = c.id
		WHERE d.name = ?
	`

	var device DeviceData
	var hostname, port, serialDevice, aprsCallsign sql.NullString
	var tlsCertFile, tlsKeyFile, path sql.NullString
	var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
	var latitude, longitude, altitude sql.NullFloat64
	var aprsEnabled sql.NullBool

	// PWS Weather fields
	var pwsEnabled sql.NullBool
	var pwsStationID, pwsPassword sql.NullString
	var pwsUploadInterval sql.NullInt64

	// Weather Underground fields
	var wuEnabled sql.NullBool
	var wuStationID, wuPassword sql.NullString
	var wuUploadInterval sql.NullInt64

	// APRS additional fields
	var aprsPasscode, aprsSymbolTable, aprsSymbolCode, aprsComment sql.NullString

	// Aeris Weather fields
	var aerisEnabled sql.NullBool
	var aerisAPIClientID, aerisAPIClientSecret sql.NullString

	err := s.db.QueryRow(query, name).Scan(
		&device.ID, &device.Name, &device.Type, &device.Enabled, &hostname, &port,
		&serialDevice, &baud, &windDirCorrection,
		&baseSnowDistance, &websiteID, &latitude, &longitude, &altitude,
		&aprsEnabled, &aprsCallsign, &tlsCertFile, &tlsKeyFile, &path,
		&pwsEnabled, &pwsStationID, &pwsPassword, &pwsUploadInterval,
		&wuEnabled, &wuStationID, &wuPassword, &wuUploadInterval,
		&aprsPasscode, &aprsSymbolTable, &aprsSymbolCode, &aprsComment,
		&aerisEnabled, &aerisAPIClientID, &aerisAPIClientSecret,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("device %s not found", name)
		}
		return nil, fmt.Errorf("failed to get device %s: %w", name, err)
	}

	// Convert nullable string fields to empty strings if NULL
	if hostname.Valid {
		device.Hostname = hostname.String
	}
	if port.Valid {
		device.Port = port.String
	}
	if serialDevice.Valid {
		device.SerialDevice = serialDevice.String
	}

	// Convert nullable integer fields to zero if NULL
	if baud.Valid {
		device.Baud = int(baud.Int64)
	}
	if windDirCorrection.Valid {
		device.WindDirCorrection = int16(windDirCorrection.Int64)
	}
	if baseSnowDistance.Valid {
		device.BaseSnowDistance = int16(baseSnowDistance.Int64)
	}

	// Set website ID if present
	if websiteID.Valid {
		websiteIDInt := int(websiteID.Int64)
		device.WebsiteID = &websiteIDInt
	}

	// Set location data if present
	if latitude.Valid {
		device.Latitude = latitude.Float64
	}
	if longitude.Valid {
		device.Longitude = longitude.Float64
	}
	if altitude.Valid {
		device.Altitude = altitude.Float64
	}

	// Set APRS data
	device.APRSEnabled = aprsEnabled.Bool
	device.APRSCallsign = aprsCallsign.String

	// Set TLS and path data
	if tlsCertFile.Valid {
		device.TLSCertPath = tlsCertFile.String
	}
	if tlsKeyFile.Valid {
		device.TLSKeyPath = tlsKeyFile.String
	}
	if path.Valid {
		device.Path = path.String
	}

	// Set PWS Weather fields
	device.PWSEnabled = pwsEnabled.Bool
	if pwsStationID.Valid {
		device.PWSStationID = pwsStationID.String
	}
	if pwsPassword.Valid {
		device.PWSPassword = pwsPassword.String
	}
	if pwsUploadInterval.Valid {
		device.PWSUploadInterval = int(pwsUploadInterval.Int64)
	}

	// Set Weather Underground fields
	device.WUEnabled = wuEnabled.Bool
	if wuStationID.Valid {
		device.WUStationID = wuStationID.String
	}
	if wuPassword.Valid {
		device.WUPassword = wuPassword.String
	}
	if wuUploadInterval.Valid {
		device.WUUploadInterval = int(wuUploadInterval.Int64)
	}

	// Set APRS additional fields
	if aprsPasscode.Valid {
		device.APRSPasscode = aprsPasscode.String
	}
	if aprsSymbolTable.Valid {
		device.APRSSymbolTable = aprsSymbolTable.String
	}
	if aprsSymbolCode.Valid {
		device.APRSSymbolCode = aprsSymbolCode.String
	}
	if aprsComment.Valid {
		device.APRSComment = aprsComment.String
	}

	// Set Aeris Weather fields
	device.AerisEnabled = aerisEnabled.Bool
	if aerisAPIClientID.Valid {
		device.AerisAPIClientID = aerisAPIClientID.String
	}
	if aerisAPIClientSecret.Valid {
		device.AerisAPIClientSecret = aerisAPIClientSecret.String
	}

	return &device, nil
}

// AddDevice adds a new device to the configuration
func (s *SQLiteProvider) AddDevice(device *DeviceData) error {
	// Validate device doesn't already exist
	if _, err := s.GetDevice(device.Name); err == nil {
		return fmt.Errorf("device %s already exists", device.Name)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get or create config ID
	configID, err := s.getOrCreateConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Insert device
	query := `
		INSERT INTO devices (
			config_id, name, type, enabled, hostname, port, serial_device,
			baud, wind_dir_correction, base_snow_distance, website_id,
			latitude, longitude, altitude, aprs_enabled, aprs_callsign,
			tls_cert_file, tls_key_file, path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var websiteID sql.NullInt64
	if device.WebsiteID != nil {
		websiteID = sql.NullInt64{Int64: int64(*device.WebsiteID), Valid: true}
	}

	result, err := tx.Exec(query,
		configID, device.Name, device.Type, device.Enabled, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		websiteID, device.Latitude, device.Longitude, device.Altitude,
		device.APRSEnabled, device.APRSCallsign,
		nullString(device.TLSCertPath), nullString(device.TLSKeyPath), nullString(device.Path),
	)
	if err != nil {
		return fmt.Errorf("failed to insert device: %w", err)
	}

	// Get the inserted ID
	deviceID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted device ID: %w", err)
	}
	device.ID = int(deviceID)

	return tx.Commit()
}

// UpdateDevice updates an existing device
func (s *SQLiteProvider) UpdateDevice(name string, device *DeviceData) error {
	// Validate device exists
	if _, err := s.GetDevice(name); err != nil {
		return fmt.Errorf("device %s not found: %w", name, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update device
	query := `
		UPDATE devices SET
			name = ?, type = ?, enabled = ?, hostname = ?, port = ?, serial_device = ?,
			baud = ?, wind_dir_correction = ?, base_snow_distance = ?, website_id = ?,
			latitude = ?, longitude = ?, altitude = ?, aprs_enabled = ?, aprs_callsign = ?,
			tls_cert_file = ?, tls_key_file = ?, path = ?
		WHERE name = ?
	`

	var websiteID sql.NullInt64
	if device.WebsiteID != nil {
		websiteID = sql.NullInt64{Int64: int64(*device.WebsiteID), Valid: true}
	}

	_, err = tx.Exec(query,
		device.Name, device.Type, device.Enabled, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		websiteID, device.Latitude, device.Longitude, device.Altitude,
		device.APRSEnabled, device.APRSCallsign,
		nullString(device.TLSCertPath), nullString(device.TLSKeyPath), nullString(device.Path),
		name,
	)

	if err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}

	return tx.Commit()
}

// DeleteDevice removes a device from the configuration
func (s *SQLiteProvider) DeleteDevice(name string) error {
	// Validate device exists
	if _, err := s.GetDevice(name); err != nil {
		return fmt.Errorf("device %s not found: %w", name, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete device
	query := "DELETE FROM devices WHERE name = ?"
	result, err := tx.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device %s not found", name)
	}

	return tx.Commit()
}

// Individual storage management methods

// AddStorageConfig adds a new storage configuration
func (s *SQLiteProvider) AddStorageConfig(storageType string, config interface{}) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get or create config ID
	configID, err := s.getOrCreateConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Check if storage config already exists
	existingQuery := "SELECT COUNT(*) FROM storage_configs WHERE config_id = ? AND backend_type = ?"
	var count int
	err = tx.QueryRow(existingQuery, configID, storageType).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing storage config: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("storage config for %s already exists", storageType)
	}

	// Insert storage config based on type
	switch storageType {
	case "timescaledb":
		timescale, ok := config.(*TimescaleDBData)
		if !ok {
			return fmt.Errorf("invalid config type for TimescaleDB")
		}
		if err := s.insertTimescaleDBConfig(tx, configID, timescale); err != nil {
			return err
		}
	case "grpc":
		grpc, ok := config.(*GRPCData)
		if !ok {
			return fmt.Errorf("invalid config type for GRPC")
		}
		if err := s.insertGRPCConfig(tx, configID, grpc); err != nil {
			return err
		}
	case "aprs":
		return fmt.Errorf("APRS configuration is now managed separately via APRS management endpoints")
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return tx.Commit()
}

// UpdateStorageConfig updates an existing storage configuration
func (s *SQLiteProvider) UpdateStorageConfig(storageType string, config interface{}) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get config ID
	configID, err := s.getConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Delete existing storage config
	deleteQuery := "DELETE FROM storage_configs WHERE config_id = ? AND backend_type = ?"
	_, err = tx.Exec(deleteQuery, configID, storageType)
	if err != nil {
		return fmt.Errorf("failed to delete existing storage config: %w", err)
	}

	// Insert new storage config based on type
	switch storageType {
	case "timescaledb":
		timescale, ok := config.(*TimescaleDBData)
		if !ok {
			return fmt.Errorf("invalid config type for TimescaleDB")
		}
		if err := s.insertTimescaleDBConfig(tx, configID, timescale); err != nil {
			return err
		}
	case "grpc":
		grpc, ok := config.(*GRPCData)
		if !ok {
			return fmt.Errorf("invalid config type for GRPC")
		}
		if err := s.insertGRPCConfig(tx, configID, grpc); err != nil {
			return err
		}
	case "aprs":
		return fmt.Errorf("APRS configuration is now managed separately via APRS management endpoints")
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return tx.Commit()
}

// DeleteStorageConfig removes a storage configuration
func (s *SQLiteProvider) DeleteStorageConfig(storageType string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get config ID
	configID, err := s.getConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Delete storage config
	query := "DELETE FROM storage_configs WHERE config_id = ? AND backend_type = ?"
	result, err := tx.Exec(query, configID, storageType)
	if err != nil {
		return fmt.Errorf("failed to delete storage config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("storage config for %s not found", storageType)
	}

	return tx.Commit()
}

// Individual controller management methods

// GetController retrieves a specific controller by type
func (s *SQLiteProvider) GetController(controllerType string) (*ControllerData, error) {
	query := `
		SELECT cc.controller_type, cc.enabled,
		       cc.pws_api_endpoint,
		       cc.wu_api_endpoint,
		       cc.aeris_api_endpoint,
		       cc.rest_port, cc.rest_listen_addr,
		       cc.grpc_port, cc.grpc_listen_addr, cc.grpc_cert, cc.grpc_key,
		       cc.management_cert, cc.management_key, cc.management_port, cc.management_listen_addr,
		       cc.management_auth_token, cc.management_enable_cors,
		       cc.aprs_server
		FROM controller_configs cc
		JOIN configs c ON cc.config_id = c.id
		WHERE cc.controller_type = ?
	`

	var controller ControllerData
	var enabled bool
	var pwsAPIEndpoint, wuAPIEndpoint, aerisAPIEndpoint sql.NullString
	var restListenAddr sql.NullString
	var restPort sql.NullInt64
	var grpcListenAddr, grpcCert, grpcKey sql.NullString
	var grpcPort sql.NullInt64
	var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
	var mgmtPort sql.NullInt64
	var mgmtEnableCORS sql.NullBool
	var aprsServer sql.NullString

	err := s.db.QueryRow(query, controllerType).Scan(
		&controller.Type, &enabled,
		&pwsAPIEndpoint,
		&wuAPIEndpoint,
		&aerisAPIEndpoint,
		&restPort, &restListenAddr,
		&grpcPort, &grpcListenAddr, &grpcCert, &grpcKey,
		&mgmtCert, &mgmtKey, &mgmtPort, &mgmtListenAddr, &mgmtAuthToken, &mgmtEnableCORS,
		&aprsServer,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("controller %s not found", controllerType)
		}
		return nil, fmt.Errorf("failed to get controller %s: %w", controllerType, err)
	}

	// Populate controller-specific data
	// Note: enabled field is stored but not exposed in ControllerData struct

	// API endpoints
	if pwsAPIEndpoint.Valid {
		controller.PWSWeather = &PWSWeatherData{
			APIEndpoint: pwsAPIEndpoint.String,
		}
	}

	if wuAPIEndpoint.Valid {
		controller.WeatherUnderground = &WeatherUndergroundData{
			APIEndpoint: wuAPIEndpoint.String,
		}
	}

	if aerisAPIEndpoint.Valid {
		controller.AerisWeather = &AerisWeatherData{
			APIEndpoint: aerisAPIEndpoint.String,
		}
	}

	if restListenAddr.Valid || restPort.Valid {
		controller.RESTServer = &RESTServerData{
			HTTPPort:          int(restPort.Int64),
			DefaultListenAddr: restListenAddr.String,
			GRPCPort:          int(grpcPort.Int64),
			GRPCListenAddr:    grpcListenAddr.String,
			GRPCCertPath:      grpcCert.String,
			GRPCKeyPath:       grpcKey.String,
		}
	}

	// Management API configuration
	if mgmtListenAddr.Valid || mgmtPort.Valid || mgmtCert.Valid {
		controller.ManagementAPI = &ManagementAPIData{
			Port:       int(mgmtPort.Int64),
			ListenAddr: mgmtListenAddr.String,
			Cert:       mgmtCert.String,
			Key:        mgmtKey.String,
			AuthToken:  mgmtAuthToken.String,
		}
	}

	if aprsServer.Valid || controllerType == "aprs" {
		server := aprsServer.String
		if server == "" {
			server = "noam.aprs2.net:14580" // default APRS-IS server
		}
		controller.APRS = &APRSData{
			Server: server,
		}
	}

	return &controller, nil
}

// AddController adds a new controller to the configuration
func (s *SQLiteProvider) AddController(controller *ControllerData) error {
	// Validate controller doesn't already exist
	if _, err := s.GetController(controller.Type); err == nil {
		return fmt.Errorf("controller %s already exists", controller.Type)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get or create config ID
	configID, err := s.getOrCreateConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Insert controller
	if err := s.insertController(tx, configID, controller); err != nil {
		return fmt.Errorf("failed to insert controller: %w", err)
	}

	return tx.Commit()
}

// UpdateController updates an existing controller
func (s *SQLiteProvider) UpdateController(controllerType string, controller *ControllerData) error {
	// Validate controller exists
	if _, err := s.GetController(controllerType); err != nil {
		return fmt.Errorf("controller %s not found: %w", controllerType, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get config ID
	configID, err := s.getConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Delete existing controller
	deleteQuery := "DELETE FROM controller_configs WHERE config_id = ? AND controller_type = ?"
	_, err = tx.Exec(deleteQuery, configID, controllerType)
	if err != nil {
		return fmt.Errorf("failed to delete existing controller: %w", err)
	}

	// Insert updated controller
	if err := s.insertController(tx, configID, controller); err != nil {
		return fmt.Errorf("failed to insert updated controller: %w", err)
	}

	return tx.Commit()
}

// DeleteController removes a controller from the configuration
func (s *SQLiteProvider) DeleteController(controllerType string) error {
	// Validate controller exists
	if _, err := s.GetController(controllerType); err != nil {
		return fmt.Errorf("controller %s not found: %w", controllerType, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get config ID
	configID, err := s.getConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Delete controller (cascade will handle weather_site_configs)
	query := "DELETE FROM controller_configs WHERE config_id = ? AND controller_type = ?"
	result, err := tx.Exec(query, configID, controllerType)
	if err != nil {
		return fmt.Errorf("failed to delete controller: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("controller %s not found", controllerType)
	}

	return tx.Commit()
}

// Helper methods

// getConfigID gets the existing config ID
func (s *SQLiteProvider) getConfigID(tx *sql.Tx) (int64, error) {
	var configID int64
	err := tx.QueryRow("SELECT id FROM configs ORDER BY id LIMIT 1").Scan(&configID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no configuration found")
		}
		return 0, err
	}
	return configID, nil
}

// getOrCreateConfigID gets existing config ID or creates a new one
func (s *SQLiteProvider) getOrCreateConfigID(tx *sql.Tx) (int64, error) {
	configID, err := s.getConfigID(tx)
	if err != nil {
		// Create default config if it doesn't exist
		configID, err = s.insertConfig(tx, "default")
		if err != nil {
			return 0, fmt.Errorf("failed to create default config: %w", err)
		}
	}
	return configID, nil
}

// Weather website management methods

// GetWeatherWebsites retrieves all weather websites
func (s *SQLiteProvider) GetWeatherWebsites() ([]WeatherWebsiteData, error) {
	query := `
		SELECT w.id, w.name, w.device_id, d.name as device_name, w.hostname, w.page_title,
		       w.about_station_html, w.snow_enabled, w.snow_device_name,
		       w.air_quality_enabled, w.air_quality_device_name,
		       w.tls_cert_path, w.tls_key_path, w.is_portal, w.apple_app_id
		FROM weather_websites w
		LEFT JOIN devices d ON w.device_id = d.id
		WHERE w.config_id = (SELECT id FROM configs WHERE name = 'default')
		ORDER BY w.name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query weather websites: %w", err)
	}
	defer rows.Close()

	var websites []WeatherWebsiteData
	for rows.Next() {
		var website WeatherWebsiteData
		var deviceID sql.NullInt64
		var deviceName, hostname, pageTitle, aboutHTML, snowDeviceName sql.NullString
		var snowEnabled sql.NullBool
		var airQualityEnabled sql.NullBool
		var airQualityDeviceName sql.NullString
		var tlsCertPath, tlsKeyPath, appleAppID sql.NullString

		err := rows.Scan(
			&website.ID,
			&website.Name,
			&deviceID,
			&deviceName,
			&hostname,
			&pageTitle,
			&aboutHTML,
			&snowEnabled,
			&snowDeviceName,
			&airQualityEnabled,
			&airQualityDeviceName,
			&tlsCertPath,
			&tlsKeyPath,
			&website.IsPortal,
			&appleAppID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan website row: %w", err)
		}

		// Handle device ID and name
		if deviceID.Valid {
			deviceIDInt := int(deviceID.Int64)
			website.DeviceID = &deviceIDInt
		}
		website.DeviceName = deviceName.String
		website.Hostname = hostname.String
		website.PageTitle = pageTitle.String
		website.AboutStationHTML = aboutHTML.String
		if snowEnabled.Valid {
			website.SnowEnabled = snowEnabled.Bool
		}
		website.SnowDeviceName = snowDeviceName.String
		if airQualityEnabled.Valid {
			website.AirQualityEnabled = airQualityEnabled.Bool
		}
		website.AirQualityDeviceName = airQualityDeviceName.String
		website.TLSCertPath = tlsCertPath.String
		website.TLSKeyPath = tlsKeyPath.String
		website.AppleAppID = appleAppID.String

		websites = append(websites, website)
	}

	return websites, nil
}

// GetWeatherWebsite retrieves a specific weather website by ID
func (s *SQLiteProvider) GetWeatherWebsite(id int) (*WeatherWebsiteData, error) {
	query := `
		SELECT w.id, w.name, w.device_id, d.name as device_name, w.hostname, w.page_title,
		       w.about_station_html, w.snow_enabled, w.snow_device_name,
		       w.air_quality_enabled, w.air_quality_device_name,
		       w.tls_cert_path, w.tls_key_path, w.is_portal, w.apple_app_id
		FROM weather_websites w
		LEFT JOIN devices d ON w.device_id = d.id
		WHERE w.id = ? AND w.config_id = (SELECT id FROM configs WHERE name = 'default')`

	var website WeatherWebsiteData
	var deviceID sql.NullInt64
	var deviceName, hostname, pageTitle, aboutHTML, snowDeviceName sql.NullString
	var snowEnabled sql.NullBool
	var airQualityEnabled sql.NullBool
	var airQualityDeviceName sql.NullString
	var tlsCertPath, tlsKeyPath, appleAppID sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&website.ID,
		&website.Name,
		&deviceID,
		&deviceName,
		&hostname,
		&pageTitle,
		&aboutHTML,
		&snowEnabled,
		&snowDeviceName,
		&airQualityEnabled,
		&airQualityDeviceName,
		&tlsCertPath,
		&tlsKeyPath,
		&website.IsPortal,
		&appleAppID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("weather website %d not found", id)
		}
		return nil, fmt.Errorf("failed to get weather website %d: %w", id, err)
	}

	// Handle device ID and name
	if deviceID.Valid {
		deviceIDInt := int(deviceID.Int64)
		website.DeviceID = &deviceIDInt
	}
	website.DeviceName = deviceName.String
	website.Hostname = hostname.String
	website.PageTitle = pageTitle.String
	website.AboutStationHTML = aboutHTML.String
	if snowEnabled.Valid {
		website.SnowEnabled = snowEnabled.Bool
	}
	website.SnowDeviceName = snowDeviceName.String
	if airQualityEnabled.Valid {
		website.AirQualityEnabled = airQualityEnabled.Bool
	}
	website.AirQualityDeviceName = airQualityDeviceName.String
	website.TLSCertPath = tlsCertPath.String
	website.TLSKeyPath = tlsKeyPath.String
	website.AppleAppID = appleAppID.String

	return &website, nil
}

// AddWeatherWebsite adds a new weather website to the configuration
func (s *SQLiteProvider) AddWeatherWebsite(website *WeatherWebsiteData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get or create config ID
	configID, err := s.getOrCreateConfigID(tx)
	if err != nil {
		return fmt.Errorf("failed to get config ID: %w", err)
	}

	// Check if website name already exists
	checkQuery := `
		SELECT COUNT(*) FROM weather_websites 
		WHERE config_id = ? AND name = ?
	`
	var count int
	err = tx.QueryRow(checkQuery, configID, website.Name).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing website: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("weather website %s already exists", website.Name)
	}

	// Insert website
	insertQuery := `
		INSERT INTO weather_websites (
			config_id, name, device_id, hostname, page_title, about_station_html,
			snow_enabled, snow_device_name, air_quality_enabled, air_quality_device_name,
			tls_cert_path, tls_key_path, is_portal, apple_app_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var deviceID sql.NullInt64
	if website.DeviceID != nil {
		deviceID = sql.NullInt64{Int64: int64(*website.DeviceID), Valid: true}
	}

	result, err := tx.Exec(insertQuery,
		configID,
		website.Name,
		deviceID,
		nullString(website.Hostname),
		nullString(website.PageTitle),
		nullString(website.AboutStationHTML),
		website.SnowEnabled,
		nullString(website.SnowDeviceName),
		website.AirQualityEnabled,
		nullString(website.AirQualityDeviceName),
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
		website.IsPortal,
		nullString(website.AppleAppID),
	)
	if err != nil {
		return fmt.Errorf("failed to insert weather website: %w", err)
	}

	// Get the inserted ID
	websiteID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted website ID: %w", err)
	}
	website.ID = int(websiteID)

	return tx.Commit()
}

// UpdateWeatherWebsite updates an existing weather website
func (s *SQLiteProvider) UpdateWeatherWebsite(id int, website *WeatherWebsiteData) error {
	// Validate website exists
	if _, err := s.GetWeatherWebsite(id); err != nil {
		return fmt.Errorf("weather website %d not found: %w", id, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update website
	query := `
		UPDATE weather_websites SET
			name = ?, device_id = ?, hostname = ?, page_title = ?, about_station_html = ?,
			snow_enabled = ?, snow_device_name = ?, air_quality_enabled = ?, air_quality_device_name = ?,
			tls_cert_path = ?, tls_key_path = ?, is_portal = ?, apple_app_id = ?
		WHERE id = ?
	`

	var deviceID sql.NullInt64
	if website.DeviceID != nil {
		deviceID = sql.NullInt64{Int64: int64(*website.DeviceID), Valid: true}
	}

	_, err = tx.Exec(query,
		website.Name,
		deviceID,
		nullString(website.Hostname),
		nullString(website.PageTitle),
		nullString(website.AboutStationHTML),
		website.SnowEnabled,
		nullString(website.SnowDeviceName),
		website.AirQualityEnabled,
		nullString(website.AirQualityDeviceName),
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
		website.IsPortal,
		nullString(website.AppleAppID),
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update weather website: %w", err)
	}

	website.ID = id
	return tx.Commit()
}

// DeleteWeatherWebsite removes a weather website from the configuration
func (s *SQLiteProvider) DeleteWeatherWebsite(id int) error {
	// Validate website exists
	if _, err := s.GetWeatherWebsite(id); err != nil {
		return fmt.Errorf("weather website %d not found: %w", id, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if any devices reference this website
	deviceCheckQuery := "SELECT COUNT(*) FROM devices WHERE website_id = ?"
	var deviceCount int
	err = tx.QueryRow(deviceCheckQuery, id).Scan(&deviceCount)
	if err != nil {
		return fmt.Errorf("failed to check device references: %w", err)
	}

	if deviceCount > 0 {
		return fmt.Errorf("cannot delete website %d: %d device(s) still reference it", id, deviceCount)
	}

	// Delete website
	deleteQuery := "DELETE FROM weather_websites WHERE id = ?"
	result, err := tx.Exec(deleteQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete weather website: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("weather website %d not found", id)
	}

	return tx.Commit()
}

// Helper functions for handling nullable fields
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
