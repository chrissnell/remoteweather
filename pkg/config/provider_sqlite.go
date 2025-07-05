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
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

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
CREATE INDEX idx_storage_configs_config_id ON storage_configs(config_id);
CREATE INDEX idx_storage_configs_type ON storage_configs(config_id, backend_type);
CREATE INDEX idx_storage_configs_health_status ON storage_configs(health_status);
CREATE INDEX idx_storage_configs_health_last_check ON storage_configs(health_last_check);
CREATE INDEX idx_controller_configs_config_id ON controller_configs(config_id);
CREATE INDEX idx_controller_configs_type ON controller_configs(config_id, controller_type);
CREATE INDEX idx_weather_websites_config_id ON weather_websites(config_id);

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
		       latitude, longitude, altitude, aprs_enabled, aprs_callsign
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
		var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
		var latitude, longitude, altitude sql.NullFloat64
		var aprsEnabled sql.NullBool

		err := rows.Scan(
			&device.ID, &device.Name, &device.Type, &device.Enabled, &hostname, &port,
			&serialDevice, &baud, &windDirCorrection,
			&baseSnowDistance, &websiteID, &latitude, &longitude, &altitude,
			&aprsEnabled, &aprsCallsign,
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

		// Set APRS data
		device.APRSEnabled = aprsEnabled.Bool
		device.APRSCallsign = aprsCallsign.String

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

		// Create health data for this backend
		health := createHealthData(healthLastCheck, healthStatus, healthMessage, healthError)

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
					Health:   health,
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
					Health:         health,
				}
			}
		case "aprs":
			if aprsServer.Valid {
				storage.APRS = &APRSData{
					Server: aprsServer.String,
					Health: health,
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
		       -- PWS Weather fields
		       cc.pws_station_id, cc.pws_api_key, cc.pws_upload_interval, 
		       cc.pws_pull_from_device, cc.pws_api_endpoint,
		       -- Weather Underground fields
		       cc.wu_station_id, cc.wu_api_key, cc.wu_upload_interval,
		       cc.wu_pull_from_device, cc.wu_api_endpoint,
		       -- Aeris Weather fields
		       cc.aeris_api_client_id, cc.aeris_api_client_secret,
		       		cc.aeris_api_endpoint, cc.aeris_latitude, cc.aeris_longitude,
		       -- REST Server fields
		       cc.rest_cert, cc.rest_key, cc.rest_port, cc.rest_listen_addr,
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
		var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
		var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
		var aerisClientID, aerisClientSecret, aerisAPIEndpoint sql.NullString
		var aerisLatitude, aerisLongitude sql.NullFloat64
		var restCert, restKey, restListenAddr sql.NullString
		var restPort sql.NullInt64
		var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
		var mgmtPort sql.NullInt64
		var mgmtEnableCORS sql.NullBool
		var aprsServer sql.NullString

		err := rows.Scan(
			&controllerType, &enabled,
			&pwsStationID, &pwsAPIKey, &pwsUploadInterval, &pwsPullFromDevice, &pwsAPIEndpoint,
			&wuStationID, &wuAPIKey, &wuUploadInterval, &wuPullFromDevice, &wuAPIEndpoint,
			&aerisClientID, &aerisClientSecret, &aerisAPIEndpoint, &aerisLatitude, &aerisLongitude,
			&restCert, &restKey, &restPort, &restListenAddr,
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
			if pwsStationID.Valid {
				controller.PWSWeather = &PWSWeatherData{
					StationID:      pwsStationID.String,
					APIKey:         pwsAPIKey.String,
					UploadInterval: pwsUploadInterval.String,
					PullFromDevice: pwsPullFromDevice.String,
					APIEndpoint:    pwsAPIEndpoint.String,
				}
			}
		case "weatherunderground":
			if wuStationID.Valid {
				controller.WeatherUnderground = &WeatherUndergroundData{
					StationID:      wuStationID.String,
					APIKey:         wuAPIKey.String,
					UploadInterval: wuUploadInterval.String,
					PullFromDevice: wuPullFromDevice.String,
					APIEndpoint:    wuAPIEndpoint.String,
				}
			}
		case "aerisweather":
			if aerisClientID.Valid {
				controller.AerisWeather = &AerisWeatherData{
					APIClientID:     aerisClientID.String,
					APIClientSecret: aerisClientSecret.String,
					APIEndpoint:     aerisAPIEndpoint.String,
					Latitude:        aerisLatitude.Float64,
					Longitude:       aerisLongitude.Float64,
				}
			}
		case "rest":
			if restPort.Valid || restListenAddr.Valid || restCert.Valid || restKey.Valid {
				controller.RESTServer = &RESTServerData{
					HTTPPort:          int(restPort.Int64),
					DefaultListenAddr: restListenAddr.String,
					TLSCertPath:       restCert.String,
					TLSKeyPath:        restKey.String,
				}
				// Set HTTPS port if TLS is configured
				if restCert.Valid && restKey.Valid {
					httpsPort := int(restPort.Int64) + 1
					controller.RESTServer.HTTPSPort = &httpsPort
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
			latitude, longitude, altitude, aprs_enabled, aprs_callsign
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	query := `
		INSERT INTO controller_configs (
			config_id, controller_type, enabled,
			pws_station_id, pws_api_key, pws_upload_interval, pws_pull_from_device, pws_api_endpoint,
			wu_station_id, wu_api_key, wu_upload_interval, wu_pull_from_device, wu_api_endpoint,
			aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint, aeris_latitude, aeris_longitude,
			rest_cert, rest_key, rest_port, rest_listen_addr,
			management_cert, management_key, management_port, management_listen_addr,
			management_auth_token, management_enable_cors, aprs_server
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
	var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
	var aerisClientID, aerisClientSecret, aerisAPIEndpoint sql.NullString
	var aerisLatitude, aerisLongitude sql.NullFloat64
	var restCert, restKey, restListenAddr sql.NullString
	var restPort sql.NullInt64
	var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
	var mgmtPort sql.NullInt64
	var mgmtEnableCORS sql.NullBool
	var aprsServer sql.NullString

	if controller.PWSWeather != nil {
		pwsStationID = sql.NullString{String: controller.PWSWeather.StationID, Valid: controller.PWSWeather.StationID != ""}
		pwsAPIKey = sql.NullString{String: controller.PWSWeather.APIKey, Valid: controller.PWSWeather.APIKey != ""}
		pwsUploadInterval = sql.NullString{String: controller.PWSWeather.UploadInterval, Valid: controller.PWSWeather.UploadInterval != ""}
		pwsPullFromDevice = sql.NullString{String: controller.PWSWeather.PullFromDevice, Valid: controller.PWSWeather.PullFromDevice != ""}
		pwsAPIEndpoint = sql.NullString{String: controller.PWSWeather.APIEndpoint, Valid: controller.PWSWeather.APIEndpoint != ""}
	}

	if controller.WeatherUnderground != nil {
		wuStationID = sql.NullString{String: controller.WeatherUnderground.StationID, Valid: controller.WeatherUnderground.StationID != ""}
		wuAPIKey = sql.NullString{String: controller.WeatherUnderground.APIKey, Valid: controller.WeatherUnderground.APIKey != ""}
		wuUploadInterval = sql.NullString{String: controller.WeatherUnderground.UploadInterval, Valid: controller.WeatherUnderground.UploadInterval != ""}
		wuPullFromDevice = sql.NullString{String: controller.WeatherUnderground.PullFromDevice, Valid: controller.WeatherUnderground.PullFromDevice != ""}
		wuAPIEndpoint = sql.NullString{String: controller.WeatherUnderground.APIEndpoint, Valid: controller.WeatherUnderground.APIEndpoint != ""}
	}

	if controller.AerisWeather != nil {
		aerisClientID = sql.NullString{String: controller.AerisWeather.APIClientID, Valid: controller.AerisWeather.APIClientID != ""}
		aerisClientSecret = sql.NullString{String: controller.AerisWeather.APIClientSecret, Valid: controller.AerisWeather.APIClientSecret != ""}
		aerisAPIEndpoint = sql.NullString{String: controller.AerisWeather.APIEndpoint, Valid: controller.AerisWeather.APIEndpoint != ""}
		aerisLatitude = sql.NullFloat64{Float64: controller.AerisWeather.Latitude, Valid: controller.AerisWeather.Latitude != 0}
		aerisLongitude = sql.NullFloat64{Float64: controller.AerisWeather.Longitude, Valid: controller.AerisWeather.Longitude != 0}
	}

	if controller.RESTServer != nil {
		restCert = sql.NullString{String: controller.RESTServer.TLSCertPath, Valid: controller.RESTServer.TLSCertPath != ""}
		restKey = sql.NullString{String: controller.RESTServer.TLSKeyPath, Valid: controller.RESTServer.TLSKeyPath != ""}
		restPort = sql.NullInt64{Int64: int64(controller.RESTServer.HTTPPort), Valid: controller.RESTServer.HTTPPort != 0}
		restListenAddr = sql.NullString{String: controller.RESTServer.DefaultListenAddr, Valid: controller.RESTServer.DefaultListenAddr != ""}
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
		pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint,
		wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint,
		aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLatitude, aerisLongitude,
		restCert, restKey, restPort, restListenAddr,
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
		       d.latitude, d.longitude, d.altitude, d.aprs_enabled, d.aprs_callsign
		FROM devices d
		JOIN configs c ON d.config_id = c.id
		WHERE d.name = ?
	`

	var device DeviceData
	var hostname, port, serialDevice, aprsCallsign sql.NullString
	var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
	var latitude, longitude, altitude sql.NullFloat64
	var aprsEnabled sql.NullBool

	err := s.db.QueryRow(query, name).Scan(
		&device.ID, &device.Name, &device.Type, &device.Enabled, &hostname, &port,
		&serialDevice, &baud, &windDirCorrection,
		&baseSnowDistance, &websiteID, &latitude, &longitude, &altitude,
		&aprsEnabled, &aprsCallsign,
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
			latitude, longitude, altitude, aprs_enabled, aprs_callsign
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			latitude = ?, longitude = ?, altitude = ?, aprs_enabled = ?, aprs_callsign = ?
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
		SELECT controller_type,
		       pws_station_id, pws_api_key, pws_upload_interval, pws_pull_from_device, pws_api_endpoint,
		       wu_station_id, wu_api_key, wu_upload_interval, wu_pull_from_device, wu_api_endpoint,
		       aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint, aeris_latitude, aeris_longitude,
		       rest_cert, rest_key, rest_port, rest_listen_addr, aprs_server
		FROM controller_configs cc
		JOIN configs c ON cc.config_id = c.id
		WHERE cc.controller_type = ?
	`

	var controller ControllerData
	var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
	var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
	var aerisClientID, aerisClientSecret, aerisAPIEndpoint sql.NullString
	var aerisLatitude, aerisLongitude sql.NullFloat64
	var restCert, restKey, restListenAddr sql.NullString
	var restPort sql.NullInt64
	var aprsServer sql.NullString

	err := s.db.QueryRow(query, controllerType).Scan(
		&controller.Type,
		&pwsStationID, &pwsAPIKey, &pwsUploadInterval, &pwsPullFromDevice, &pwsAPIEndpoint,
		&wuStationID, &wuAPIKey, &wuUploadInterval, &wuPullFromDevice, &wuAPIEndpoint,
		&aerisClientID, &aerisClientSecret, &aerisAPIEndpoint, &aerisLatitude, &aerisLongitude,
		&restCert, &restKey, &restPort, &restListenAddr, &aprsServer,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("controller %s not found", controllerType)
		}
		return nil, fmt.Errorf("failed to get controller %s: %w", controllerType, err)
	}

	// Populate controller-specific data
	if pwsStationID.Valid {
		controller.PWSWeather = &PWSWeatherData{
			StationID:      pwsStationID.String,
			APIKey:         pwsAPIKey.String,
			UploadInterval: pwsUploadInterval.String,
			PullFromDevice: pwsPullFromDevice.String,
			APIEndpoint:    pwsAPIEndpoint.String,
		}
	}

	if wuStationID.Valid {
		controller.WeatherUnderground = &WeatherUndergroundData{
			StationID:      wuStationID.String,
			APIKey:         wuAPIKey.String,
			UploadInterval: wuUploadInterval.String,
			PullFromDevice: wuPullFromDevice.String,
			APIEndpoint:    wuAPIEndpoint.String,
		}
	}

	if aerisClientID.Valid {
		controller.AerisWeather = &AerisWeatherData{
			APIClientID:     aerisClientID.String,
			APIClientSecret: aerisClientSecret.String,
			APIEndpoint:     aerisAPIEndpoint.String,
			Latitude:        aerisLatitude.Float64,
			Longitude:       aerisLongitude.Float64,
		}
	}

	if restListenAddr.Valid || restPort.Valid || restCert.Valid {
		controller.RESTServer = &RESTServerData{
			HTTPPort:          int(restPort.Int64),
			DefaultListenAddr: restListenAddr.String,
			TLSCertPath:       restCert.String,
			TLSKeyPath:        restKey.String,
		}

		// Set HTTPS port if configured (would come from a separate field in future)
		// For now, we assume HTTPS is on HTTPPort + 1 if TLS is configured
		if restCert.Valid && restKey.Valid {
			httpsPort := int(restPort.Int64) + 1
			controller.RESTServer.HTTPSPort = &httpsPort
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
		       w.about_station_html, w.snow_enabled, w.snow_device_name, w.tls_cert_path, 
		       w.tls_key_path, w.is_portal 
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
		var tlsCertPath, tlsKeyPath sql.NullString

		err := rows.Scan(
			&website.ID,
			&website.Name,
			&deviceID,
			&deviceName,
			&hostname,
			&pageTitle,
			&aboutHTML,
			&website.SnowEnabled,
			&snowDeviceName,
			&tlsCertPath,
			&tlsKeyPath,
			&website.IsPortal,
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
		website.SnowDeviceName = snowDeviceName.String
		website.TLSCertPath = tlsCertPath.String
		website.TLSKeyPath = tlsKeyPath.String

		websites = append(websites, website)
	}

	return websites, nil
}

// GetWeatherWebsite retrieves a specific weather website by ID
func (s *SQLiteProvider) GetWeatherWebsite(id int) (*WeatherWebsiteData, error) {
	query := `
		SELECT w.id, w.name, w.device_id, d.name as device_name, w.hostname, w.page_title, 
		       w.about_station_html, w.snow_enabled, w.snow_device_name, w.tls_cert_path, 
		       w.tls_key_path, w.is_portal 
		FROM weather_websites w
		LEFT JOIN devices d ON w.device_id = d.id
		WHERE w.id = ? AND w.config_id = (SELECT id FROM configs WHERE name = 'default')`

	var website WeatherWebsiteData
	var deviceID sql.NullInt64
	var deviceName, hostname, pageTitle, aboutHTML, snowDeviceName sql.NullString
	var tlsCertPath, tlsKeyPath sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&website.ID,
		&website.Name,
		&deviceID,
		&deviceName,
		&hostname,
		&pageTitle,
		&aboutHTML,
		&website.SnowEnabled,
		&snowDeviceName,
		&tlsCertPath,
		&tlsKeyPath,
		&website.IsPortal,
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
	website.SnowDeviceName = snowDeviceName.String
	website.TLSCertPath = tlsCertPath.String
	website.TLSKeyPath = tlsKeyPath.String

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
			snow_enabled, snow_device_name, tls_cert_path, tls_key_path, is_portal
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
		website.IsPortal,
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
			snow_enabled = ?, snow_device_name = ?, tls_cert_path = ?, tls_key_path = ?, is_portal = ?
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
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
		website.IsPortal,
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

func nullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

// createHealthData creates a StorageHealthData from database fields
func createHealthData(lastCheck sql.NullTime, status, message, error sql.NullString) *StorageHealthData {
	if !lastCheck.Valid && !status.Valid && !message.Valid && !error.Valid {
		return nil // No health data available
	}

	health := &StorageHealthData{}
	if lastCheck.Valid {
		health.LastCheck = lastCheck.Time
	}
	if status.Valid {
		health.Status = status.String
	}
	if message.Valid {
		health.Message = message.String
	}
	if error.Valid {
		health.Error = error.String
	}
	return health
}

// Storage Health Management Implementation

// UpdateStorageHealth updates the health status of a storage backend
func (s *SQLiteProvider) UpdateStorageHealth(storageType string, health *StorageHealthData) error {
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

	// Update health status
	updateQuery := `
		UPDATE storage_configs SET
			health_last_check = ?,
			health_status = ?,
			health_message = ?,
			health_error = ?
		WHERE config_id = ? AND backend_type = ?
	`

	var lastCheck interface{}
	if !health.LastCheck.IsZero() {
		lastCheck = health.LastCheck.UTC().Format("2006-01-02 15:04:05")
	}

	_, err = tx.Exec(updateQuery,
		lastCheck,
		nullString(health.Status).String,
		nullString(health.Message).String,
		nullString(health.Error).String,
		configID,
		storageType,
	)
	if err != nil {
		return fmt.Errorf("failed to update storage health: %w", err)
	}

	return tx.Commit()
}

// GetStorageHealth retrieves the health status of a specific storage backend
func (s *SQLiteProvider) GetStorageHealth(storageType string) (*StorageHealthData, error) {
	query := `
		SELECT health_last_check, health_status, health_message, health_error
		FROM storage_configs 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default') 
		  AND backend_type = ?
	`

	var healthLastCheck sql.NullTime
	var healthStatus, healthMessage, healthError sql.NullString

	err := s.db.QueryRow(query, storageType).Scan(
		&healthLastCheck, &healthStatus, &healthMessage, &healthError,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("storage backend '%s' not found", storageType)
		}
		return nil, fmt.Errorf("failed to get storage health: %w", err)
	}

	return createHealthData(healthLastCheck, healthStatus, healthMessage, healthError), nil
}

// GetAllStorageHealth retrieves health status for all storage backends
func (s *SQLiteProvider) GetAllStorageHealth() (map[string]*StorageHealthData, error) {
	query := `
		SELECT backend_type, health_last_check, health_status, health_message, health_error
		FROM storage_configs 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default') 
		  AND enabled = 1
		ORDER BY backend_type
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query storage health: %w", err)
	}
	defer rows.Close()

	healthMap := make(map[string]*StorageHealthData)

	for rows.Next() {
		var backendType string
		var healthLastCheck sql.NullTime
		var healthStatus, healthMessage, healthError sql.NullString

		err := rows.Scan(
			&backendType,
			&healthLastCheck, &healthStatus, &healthMessage, &healthError,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan storage health row: %w", err)
		}

		healthMap[backendType] = createHealthData(healthLastCheck, healthStatus, healthMessage, healthError)
	}

	return healthMap, nil
}
