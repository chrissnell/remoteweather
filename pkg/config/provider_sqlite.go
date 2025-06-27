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
		       wind_dir_correction, base_snow_distance,
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
		var solarLat, solarLon, solarAlt sql.NullFloat64

		err := rows.Scan(
			&device.Name, &device.Type, &device.Hostname, &device.Port,
			&device.SerialDevice, &device.Baud, &device.WindDirCorrection,
			&device.BaseSnowDistance, &solarLat, &solarLon, &solarAlt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
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
		case "aprs":
			if aprsCallsign.Valid {
				storage.APRS = &APRSData{
					Callsign:     aprsCallsign.String,
					Passcode:     aprsPasscode.String,
					APRSISServer: aprsServer.String,
					Location: PointData{
						Lat: aprsLat.Float64,
						Lon: aprsLon.Float64,
					},
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
		       cc.aeris_api_endpoint, cc.aeris_location,
		       -- REST Server fields
		       cc.rest_cert, cc.rest_key, cc.rest_port, cc.rest_listen_addr,
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
		var wsStationName, wsPullFromDevice, wsSnowDevice, wsPageTitle, wsAboutHTML sql.NullString
		var wsSnowEnabled sql.NullBool
		var wsSnowBaseDistance sql.NullFloat64

		err := rows.Scan(
			&controllerType, &enabled,
			&pwsStationID, &pwsAPIKey, &pwsUploadInterval, &pwsPullFromDevice, &pwsAPIEndpoint,
			&wuStationID, &wuAPIKey, &wuUploadInterval, &wuPullFromDevice, &wuAPIEndpoint,
			&aerisClientID, &aerisClientSecret, &aerisAPIEndpoint, &aerisLocation,
			&restCert, &restKey, &restPort, &restListenAddr,
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
			if restPort.Valid {
				controller.RESTServer = &RESTServerData{
					Cert:       restCert.String,
					Key:        restKey.String,
					Port:       int(restPort.Int64),
					ListenAddr: restListenAddr.String,
				}

				// Add weather site config if present
				if wsStationName.Valid {
					controller.RESTServer.WeatherSiteConfig = WeatherSiteData{
						StationName:      wsStationName.String,
						PullFromDevice:   wsPullFromDevice.String,
						SnowEnabled:      wsSnowEnabled.Bool,
						SnowDevice:       wsSnowDevice.String,
						SnowBaseDistance: float32(wsSnowBaseDistance.Float64),
						PageTitle:        wsPageTitle.String,
						AboutStationHTML: wsAboutHTML.String,
					}
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
			baud, wind_dir_correction, base_snow_distance,
			solar_latitude, solar_longitude, solar_altitude
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.Exec(query,
		configID, device.Name, device.Type, device.Hostname, device.Port,
		device.SerialDevice, device.Baud, device.WindDirCorrection, device.BaseSnowDistance,
		device.Solar.Latitude, device.Solar.Longitude, device.Solar.Altitude,
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

	if storage.APRS != nil {
		if err := s.insertAPRSConfig(tx, configID, storage.APRS); err != nil {
			return err
		}
	}

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

func (s *SQLiteProvider) insertAPRSConfig(tx *sql.Tx, configID int64, aprs *APRSData) error {
	query := `
		INSERT INTO storage_configs (
			config_id, backend_type, enabled,
			aprs_callsign, aprs_passcode, aprs_server, aprs_location_lat, aprs_location_lon
		) VALUES (?, 'aprs', 1, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query, configID,
		aprs.Callsign, aprs.Passcode, aprs.APRSISServer,
		aprs.Location.Lat, aprs.Location.Lon,
	)
	return err
}

func (s *SQLiteProvider) insertController(tx *sql.Tx, configID int64, controller *ControllerData) error {
	// Insert controller record
	query := `
		INSERT INTO controller_configs (
			config_id, controller_type, enabled,
			pws_station_id, pws_api_key, pws_upload_interval, pws_pull_from_device, pws_api_endpoint,
			wu_station_id, wu_api_key, wu_upload_interval, wu_pull_from_device, wu_api_endpoint,
			aeris_api_client_id, aeris_api_client_secret, aeris_api_endpoint, aeris_location,
			rest_cert, rest_key, rest_port, rest_listen_addr
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint sql.NullString
	var wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint sql.NullString
	var aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation sql.NullString
	var restCert, restKey, restListenAddr sql.NullString
	var restPort sql.NullInt64

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
		restCert = sql.NullString{String: controller.RESTServer.Cert, Valid: controller.RESTServer.Cert != ""}
		restKey = sql.NullString{String: controller.RESTServer.Key, Valid: controller.RESTServer.Key != ""}
		restPort = sql.NullInt64{Int64: int64(controller.RESTServer.Port), Valid: controller.RESTServer.Port != 0}
		restListenAddr = sql.NullString{String: controller.RESTServer.ListenAddr, Valid: controller.RESTServer.ListenAddr != ""}
	}

	result, err := tx.Exec(query, configID, controller.Type,
		pwsStationID, pwsAPIKey, pwsUploadInterval, pwsPullFromDevice, pwsAPIEndpoint,
		wuStationID, wuAPIKey, wuUploadInterval, wuPullFromDevice, wuAPIEndpoint,
		aerisClientID, aerisClientSecret, aerisAPIEndpoint, aerisLocation,
		restCert, restKey, restPort, restListenAddr,
	)
	if err != nil {
		return err
	}

	// Insert weather site config if this is a REST server controller
	if controller.RESTServer != nil {
		controllerID, err := result.LastInsertId()
		if err != nil {
			return err
		}

		return s.insertWeatherSiteConfig(tx, controllerID, &controller.RESTServer.WeatherSiteConfig)
	}

	return nil
}

func (s *SQLiteProvider) insertWeatherSiteConfig(tx *sql.Tx, controllerConfigID int64, site *WeatherSiteData) error {
	query := `
		INSERT INTO weather_site_configs (
			controller_config_id, station_name, pull_from_device, snow_enabled,
			snow_device, snow_base_distance, page_title, about_station_html
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.Exec(query, controllerConfigID,
		site.StationName, site.PullFromDevice, site.SnowEnabled,
		site.SnowDevice, site.SnowBaseDistance, site.PageTitle, site.AboutStationHTML,
	)
	return err
}
