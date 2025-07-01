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

	return &SQLiteProvider{
		db:     db,
		dbPath: dbPath,
	}, nil
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
		SELECT name, type, hostname, port, serial_device, baud, 
		       wind_dir_correction, base_snow_distance, website_id,
		       solar_latitude, solar_longitude, solar_altitude
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
		var hostname, port, serialDevice sql.NullString
		var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
		var solarLat, solarLon, solarAlt sql.NullFloat64

		err := rows.Scan(
			&device.Name, &device.Type, &hostname, &port,
			&serialDevice, &baud, &windDirCorrection,
			&baseSnowDistance, &websiteID, &solarLat, &solarLon, &solarAlt,
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

		// Set solar data if present
		if solarLat.Valid && solarLon.Valid && solarAlt.Valid {
			device.Solar = SolarData{
				Latitude:  solarLat.Float64,
				Longitude: solarLon.Float64,
				Altitude:  solarAlt.Float64,
			}
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
		       timescale_connection_string,
		       -- gRPC fields
		       grpc_cert, grpc_key, grpc_listen_addr, grpc_port, grpc_pull_from_device,
		       -- APRS fields
		       aprs_callsign, aprs_passcode, aprs_server, aprs_location_lat, aprs_location_lon
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
		var timescaleConnectionString sql.NullString
		var grpcCert, grpcKey, grpcListenAddr, grpcPullFromDevice sql.NullString
		var grpcPort sql.NullInt64
		// Note: APRS fields removed - now handled by separate APRS tables
		var aprsCallsign, aprsPasscode, aprsServer sql.NullString
		var aprsLat, aprsLon sql.NullFloat64

		err := rows.Scan(
			&backendType, &enabled,
			&timescaleConnectionString,
			&grpcCert, &grpcKey, &grpcListenAddr, &grpcPort, &grpcPullFromDevice,
			&aprsCallsign, &aprsPasscode, &aprsServer, &aprsLat, &aprsLon,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan storage config row: %w", err)
		}

		switch backendType {
		case "timescaledb":
			if timescaleConnectionString.Valid {
				storage.TimescaleDB = &TimescaleDBData{
					ConnectionString: timescaleConnectionString.String,
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
			// APRS case removed - now handled by separate methods
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
		       cc.aeris_api_endpoint, cc.aeris_location,
		       -- REST Server fields
		       cc.rest_cert, cc.rest_key, cc.rest_port, cc.rest_listen_addr,
		       -- Management API fields
		       cc.management_cert, cc.management_key, cc.management_port, cc.management_listen_addr,
		       cc.management_auth_token, cc.management_enable_cors,
		       -- Weather Site fields
		       wsc.station_name, wsc.pull_from_device, wsc.snow_enabled,
		       wsc.snow_device, wsc.snow_base_distance, wsc.page_title,
		       wsc.about_station_html
		FROM controller_configs cc
		LEFT JOIN weather_site_configs wsc ON cc.id = wsc.controller_config_id
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
		var aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation sql.NullString
		var restCert, restKey, restListenAddr sql.NullString
		var restPort sql.NullInt64
		var mgmtCert, mgmtKey, mgmtListenAddr, mgmtAuthToken sql.NullString
		var mgmtPort sql.NullInt64
		var mgmtEnableCORS sql.NullBool
		var wsStationName, wsPullFromDevice, wsSnowDevice, wsPageTitle, wsAboutHTML sql.NullString
		var wsSnowEnabled sql.NullBool
		var wsSnowBaseDistance sql.NullFloat64

		err := rows.Scan(
			&controllerType, &enabled,
			&pwsStationID, &pwsAPIKey, &pwsUploadInterval, &pwsPullFromDevice, &pwsAPIEndpoint,
			&wuStationID, &wuAPIKey, &wuUploadInterval, &wuPullFromDevice, &wuAPIEndpoint,
			&aerisClientID, &aerisClientSecret, &aerisAPIEndpoint, &aerisLocation,
			&restCert, &restKey, &restPort, &restListenAddr,
			&mgmtCert, &mgmtKey, &mgmtPort, &mgmtListenAddr, &mgmtAuthToken, &mgmtEnableCORS,
			&wsStationName, &wsPullFromDevice, &wsSnowEnabled,
			&wsSnowDevice, &wsSnowBaseDistance, &wsPageTitle, &wsAboutHTML,
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
					Location:        aerisLocation.String,
				}
			}
		case "rest":
			controller.RESTServer = &RESTServerData{
				DefaultListenAddr: restListenAddr.String,
			}
			// Note: Individual website port configurations are now handled via weather_websites table
		case "management":
			if mgmtPort.Valid {
				controller.ManagementAPI = &ManagementAPIData{
					Cert:       mgmtCert.String,
					Key:        mgmtKey.String,
					Port:       int(mgmtPort.Int64),
					ListenAddr: mgmtListenAddr.String,
					AuthToken:  mgmtAuthToken.String,
					EnableCORS: mgmtEnableCORS.Bool,
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
		"DELETE FROM weather_site_configs WHERE controller_config_id IN (SELECT id FROM controller_configs WHERE config_id = ?)",
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
			config_id, name, type, hostname, port, serial_device,
			baud, wind_dir_correction, base_snow_distance, website_id,
			solar_latitude, solar_longitude, solar_altitude
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var websiteID sql.NullInt64
	if device.WebsiteID != nil {
		websiteID = sql.NullInt64{Int64: int64(*device.WebsiteID), Valid: true}
	}

	_, err := tx.Exec(query,
		configID, device.Name, device.Type, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		websiteID, device.Solar.Latitude, device.Solar.Longitude, device.Solar.Altitude,
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
			config_id, backend_type, enabled, timescale_connection_string
		) VALUES (?, 'timescaledb', 1, ?)
	`
	_, err := tx.Exec(query, configID, timescale.ConnectionString)
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
			aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint, aeris_location,
			rest_cert, rest_key, rest_port, rest_listen_addr,
			management_cert, management_key, management_port, management_listen_addr,
			management_auth_token, management_enable_cors, aprs_server
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
	var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
	var aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation sql.NullString
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
		aerisLocation = sql.NullString{String: controller.AerisWeather.Location, Valid: controller.AerisWeather.Location != ""}
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
		mgmtEnableCORS = sql.NullBool{Bool: controller.ManagementAPI.EnableCORS, Valid: true}
	}

	if controller.APRS != nil {
		aprsServer = sql.NullString{String: controller.APRS.Server, Valid: controller.APRS.Server != ""}
	}

	_, err := tx.Exec(query, configID, controller.Type,
		pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint,
		wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint,
		aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation,
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
		SELECT d.name, d.type, d.hostname, d.port, d.serial_device, d.baud,
		       d.wind_dir_correction, d.base_snow_distance, d.website_id,
		       d.solar_latitude, d.solar_longitude, d.solar_altitude
		FROM devices d
		JOIN configs c ON d.config_id = c.id
		WHERE d.name = ?
	`

	var device DeviceData
	var hostname, port, serialDevice sql.NullString
	var baud, windDirCorrection, baseSnowDistance, websiteID sql.NullInt64
	var solarLat, solarLon, solarAlt sql.NullFloat64

	err := s.db.QueryRow(query, name).Scan(
		&device.Name, &device.Type, &hostname, &port,
		&serialDevice, &baud, &windDirCorrection,
		&baseSnowDistance, &websiteID, &solarLat, &solarLon, &solarAlt,
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

	// Set solar data if present
	if solarLat.Valid && solarLon.Valid && solarAlt.Valid {
		device.Solar = SolarData{
			Latitude:  solarLat.Float64,
			Longitude: solarLon.Float64,
			Altitude:  solarAlt.Float64,
		}
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
	if err := s.insertDevice(tx, configID, device); err != nil {
		return fmt.Errorf("failed to insert device: %w", err)
	}

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
			name = ?, type = ?, hostname = ?, port = ?, serial_device = ?,
			baud = ?, wind_dir_correction = ?, base_snow_distance = ?, website_id = ?,
			solar_latitude = ?, solar_longitude = ?, solar_altitude = ?
		WHERE name = ?
	`

	var websiteID sql.NullInt64
	if device.WebsiteID != nil {
		websiteID = sql.NullInt64{Int64: int64(*device.WebsiteID), Valid: true}
	}

	_, err = tx.Exec(query,
		device.Name, device.Type, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		websiteID, device.Solar.Latitude, device.Solar.Longitude, device.Solar.Altitude,
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
		return s.insertTimescaleDBConfig(tx, configID, timescale)
	case "grpc":
		grpc, ok := config.(*GRPCData)
		if !ok {
			return fmt.Errorf("invalid config type for GRPC")
		}
		return s.insertGRPCConfig(tx, configID, grpc)
	case "aprs":
		return fmt.Errorf("APRS configuration is now managed separately via APRS management endpoints")
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
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
		       aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint, aeris_location,
		       rest_cert, rest_key, rest_port, rest_listen_addr, aprs_server
		FROM controller_configs cc
		JOIN configs c ON cc.config_id = c.id
		WHERE cc.controller_type = ?
	`

	var controller ControllerData
	var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
	var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
	var aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation sql.NullString
	var restCert, restKey, restListenAddr sql.NullString
	var restPort sql.NullInt64
	var aprsServer sql.NullString

	err := s.db.QueryRow(query, controllerType).Scan(
		&controller.Type,
		&pwsStationID, &pwsAPIKey, &pwsUploadInterval, &pwsPullFromDevice, &pwsAPIEndpoint,
		&wuStationID, &wuAPIKey, &wuUploadInterval, &wuPullFromDevice, &wuAPIEndpoint,
		&aerisClientID, &aerisClientSecret, &aerisAPIEndpoint, &aerisLocation,
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
			Location:        aerisLocation.String,
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
		SELECT id, name, hostname, page_title, about_station_html, snow_enabled, 
		       snow_device_name, tls_cert_path, tls_key_path
		FROM weather_websites 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default')
		ORDER BY name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query weather websites: %w", err)
	}
	defer rows.Close()

	var websites []WeatherWebsiteData
	for rows.Next() {
		var website WeatherWebsiteData
		var hostname, pageTitle, aboutHTML, snowDeviceName sql.NullString
		var tlsCertPath, tlsKeyPath sql.NullString

		err := rows.Scan(
			&website.ID,
			&website.Name,
			&hostname,
			&pageTitle,
			&aboutHTML,
			&website.SnowEnabled,
			&snowDeviceName,
			&tlsCertPath,
			&tlsKeyPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan website row: %w", err)
		}

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
		SELECT id, name, hostname, page_title, about_station_html, snow_enabled,
		       snow_device_name, tls_cert_path, tls_key_path
		FROM weather_websites 
		WHERE id = ? AND config_id = (SELECT id FROM configs WHERE name = 'default')
	`

	var website WeatherWebsiteData
	var hostname, pageTitle, aboutHTML, snowDeviceName sql.NullString
	var tlsCertPath, tlsKeyPath sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&website.ID,
		&website.Name,
		&hostname,
		&pageTitle,
		&aboutHTML,
		&website.SnowEnabled,
		&snowDeviceName,
		&tlsCertPath,
		&tlsKeyPath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("weather website %d not found", id)
		}
		return nil, fmt.Errorf("failed to get weather website %d: %w", id, err)
	}

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
			config_id, name, hostname, page_title, about_station_html, 
			snow_enabled, snow_device_name, tls_cert_path, tls_key_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.Exec(insertQuery,
		configID,
		website.Name,
		nullString(website.Hostname),
		nullString(website.PageTitle),
		nullString(website.AboutStationHTML),
		website.SnowEnabled,
		nullString(website.SnowDeviceName),
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
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
			name = ?, hostname = ?, page_title = ?, about_station_html = ?,
			snow_enabled = ?, snow_device_name = ?, tls_cert_path = ?, tls_key_path = ?
		WHERE id = ?
	`

	_, err = tx.Exec(query,
		website.Name,
		nullString(website.Hostname),
		nullString(website.PageTitle),
		nullString(website.AboutStationHTML),
		website.SnowEnabled,
		nullString(website.SnowDeviceName),
		nullString(website.TLSCertPath),
		nullString(website.TLSKeyPath),
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

// Station APRS Configuration Management
func (s *SQLiteProvider) GetStationAPRSConfigs() ([]StationAPRSData, error) {
	query := `
		SELECT device_name, enabled, callsign, latitude, longitude
		FROM station_aprs_configs 
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default')
		ORDER BY device_name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query station APRS configs: %w", err)
	}
	defer rows.Close()

	var configs []StationAPRSData
	for rows.Next() {
		var config StationAPRSData
		var callsign sql.NullString
		var latitude, longitude sql.NullFloat64

		err := rows.Scan(
			&config.DeviceName,
			&config.Enabled,
			&callsign,
			&latitude,
			&longitude,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan station APRS config row: %w", err)
		}

		config.Callsign = callsign.String
		config.Location.Lat = latitude.Float64
		config.Location.Lon = longitude.Float64

		configs = append(configs, config)
	}

	return configs, nil
}

func (s *SQLiteProvider) GetStationAPRSConfig(deviceName string) (*StationAPRSData, error) {
	query := `
		SELECT device_name, enabled, callsign, latitude, longitude
		FROM station_aprs_configs 
		WHERE device_name = ? AND config_id = (SELECT id FROM configs WHERE name = 'default')
	`

	var config StationAPRSData
	var callsign sql.NullString
	var latitude, longitude sql.NullFloat64

	err := s.db.QueryRow(query, deviceName).Scan(
		&config.DeviceName,
		&config.Enabled,
		&callsign,
		&latitude,
		&longitude,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("station APRS config for device '%s' not found", deviceName)
		}
		return nil, fmt.Errorf("failed to get station APRS config: %w", err)
	}

	config.Callsign = callsign.String
	config.Location.Lat = latitude.Float64
	config.Location.Lon = longitude.Float64

	return &config, nil
}

func (s *SQLiteProvider) AddStationAPRSConfig(config *StationAPRSData) error {
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

	// Verify the device exists
	deviceCheckQuery := "SELECT COUNT(*) FROM devices WHERE name = ? AND config_id = ?"
	var deviceCount int
	err = tx.QueryRow(deviceCheckQuery, config.DeviceName, configID).Scan(&deviceCount)
	if err != nil {
		return fmt.Errorf("failed to check device existence: %w", err)
	}
	if deviceCount == 0 {
		return fmt.Errorf("device '%s' does not exist", config.DeviceName)
	}

	// Check if station APRS config already exists
	existingQuery := "SELECT COUNT(*) FROM station_aprs_configs WHERE device_name = ? AND config_id = ?"
	var count int
	err = tx.QueryRow(existingQuery, config.DeviceName, configID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing station APRS config: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("station APRS config for device '%s' already exists", config.DeviceName)
	}

	// Insert new station APRS config
	insertQuery := `
		INSERT INTO station_aprs_configs (
			config_id, device_name, enabled, callsign, latitude, longitude
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(insertQuery,
		configID,
		config.DeviceName,
		config.Enabled,
		nullString(config.Callsign),
		nullFloat64(config.Location.Lat),
		nullFloat64(config.Location.Lon),
	)
	if err != nil {
		return fmt.Errorf("failed to insert station APRS config: %w", err)
	}

	return tx.Commit()
}

func (s *SQLiteProvider) UpdateStationAPRSConfig(deviceName string, config *StationAPRSData) error {
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

	// Update station APRS config
	updateQuery := `
		UPDATE station_aprs_configs SET
			enabled = ?, callsign = ?, latitude = ?, longitude = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE device_name = ? AND config_id = ?
	`

	result, err := tx.Exec(updateQuery,
		config.Enabled,
		nullString(config.Callsign),
		nullFloat64(config.Location.Lat),
		nullFloat64(config.Location.Lon),
		deviceName,
		configID,
	)
	if err != nil {
		return fmt.Errorf("failed to update station APRS config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("station APRS config for device '%s' not found", deviceName)
	}

	return tx.Commit()
}

func (s *SQLiteProvider) DeleteStationAPRSConfig(deviceName string) error {
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

	// Delete station APRS config
	deleteQuery := "DELETE FROM station_aprs_configs WHERE device_name = ? AND config_id = ?"
	result, err := tx.Exec(deleteQuery, deviceName, configID)
	if err != nil {
		return fmt.Errorf("failed to delete station APRS config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("station APRS config for device '%s' not found", deviceName)
	}

	return tx.Commit()
}
