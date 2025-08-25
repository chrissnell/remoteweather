package config

import (
	"database/sql"
	"fmt"
	"sync"
)

// RemoteStationProvider wraps a ConfigProvider to include remote stations as virtual devices
type RemoteStationProvider struct {
	provider ConfigProvider
	db       *sql.DB
	mu       sync.RWMutex
}

// NewRemoteStationProvider creates a provider that includes remote stations
func NewRemoteStationProvider(provider ConfigProvider, db *sql.DB) *RemoteStationProvider {
	return &RemoteStationProvider{
		provider: provider,
		db:       db,
	}
}

// GetDevices returns both local devices and remote stations as virtual devices
func (r *RemoteStationProvider) GetDevices() ([]DeviceData, error) {
	// Get local devices
	devices, err := r.provider.GetDevices()
	if err != nil {
		return nil, err
	}

	// Query remote stations from database
	rows, err := r.db.Query(`
		SELECT station_name, station_type,
		       aprs_enabled, aprs_callsign, aprs_password,
		       wu_enabled, wu_station_id, wu_api_key,
		       aeris_enabled, aeris_client_id, aeris_client_secret,
		       pws_enabled, pws_station_id, pws_password
		FROM remote_stations
	`)
	if err != nil {
		// If table doesn't exist, just return local devices
		return devices, nil
	}
	defer rows.Close()

	// Convert remote stations to virtual devices
	for rows.Next() {
		var stationName, stationType string
		var aprsEnabled, wuEnabled, aerisEnabled, pwsEnabled bool
		var aprsCallsign, aprsPassword, wuStationID, wuAPIKey sql.NullString
		var aerisClientID, aerisClientSecret, pwsStationID, pwsPassword sql.NullString

		err := rows.Scan(
			&stationName, &stationType,
			&aprsEnabled, &aprsCallsign, &aprsPassword,
			&wuEnabled, &wuStationID, &wuAPIKey,
			&aerisEnabled, &aerisClientID, &aerisClientSecret,
			&pwsEnabled, &pwsStationID, &pwsPassword,
		)
		if err != nil {
			continue
		}

		// Create virtual device for remote station
		device := DeviceData{
			Name:    stationName,
			Type:    stationType,
			Enabled: true,
		}

		// Add APRS config if enabled
		if aprsEnabled && aprsCallsign.Valid {
			device.APRSEnabled = true
			device.APRSCallsign = aprsCallsign.String
			device.APRSPasscode = aprsPassword.String
		}

		// Add WU config if enabled
		if wuEnabled && wuStationID.Valid {
			device.WUEnabled = true
			device.WUStationID = wuStationID.String
			device.WUPassword = wuAPIKey.String
		}

		// Add Aeris config if enabled
		if aerisEnabled && aerisClientID.Valid {
			device.AerisEnabled = true
			device.AerisAPIClientID = aerisClientID.String
			device.AerisAPIClientSecret = aerisClientSecret.String
		}

		// Add PWS config if enabled
		if pwsEnabled && pwsStationID.Valid {
			device.PWSEnabled = true
			device.PWSStationID = pwsStationID.String
			device.PWSPassword = pwsPassword.String
		}

		devices = append(devices, device)
	}

	return devices, rows.Err()
}

// LoadConfig delegates to wrapped provider
func (r *RemoteStationProvider) LoadConfig() (*ConfigData, error) {
	config, err := r.provider.LoadConfig()
	if err != nil {
		return nil, err
	}

	// Update devices list to include remote stations
	devices, err := r.GetDevices()
	if err != nil {
		return config, nil
	}
	config.Devices = devices

	return config, nil
}

// GetDevice retrieves a device by name
func (r *RemoteStationProvider) GetDevice(deviceName string) (*DeviceData, error) {
	// Check local devices first
	device, err := r.provider.GetDevice(deviceName)
	if err == nil {
		return device, nil
	}

	// Check remote stations
	devices, err := r.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		if d.Name == deviceName {
			return &d, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", deviceName)
}

// All modification methods delegate to the wrapped provider (remote stations are read-only)
func (r *RemoteStationProvider) AddDevice(device *DeviceData) error {
	return r.provider.AddDevice(device)
}

func (r *RemoteStationProvider) UpdateDevice(name string, device *DeviceData) error {
	return r.provider.UpdateDevice(name, device)
}

func (r *RemoteStationProvider) DeleteDevice(deviceName string) error {
	return r.provider.DeleteDevice(deviceName)
}

func (r *RemoteStationProvider) GetStorageConfig() (*StorageData, error) {
	return r.provider.GetStorageConfig()
}

func (r *RemoteStationProvider) GetControllers() ([]ControllerData, error) {
	return r.provider.GetControllers()
}

func (r *RemoteStationProvider) AddStorageConfig(storageType string, config interface{}) error {
	return r.provider.AddStorageConfig(storageType, config)
}

func (r *RemoteStationProvider) UpdateStorageConfig(storageType string, config interface{}) error {
	return r.provider.UpdateStorageConfig(storageType, config)
}

func (r *RemoteStationProvider) DeleteStorageConfig(storageType string) error {
	return r.provider.DeleteStorageConfig(storageType)
}

func (r *RemoteStationProvider) AddController(controller *ControllerData) error {
	return r.provider.AddController(controller)
}

func (r *RemoteStationProvider) UpdateController(controllerType string, controller *ControllerData) error {
	return r.provider.UpdateController(controllerType, controller)
}

func (r *RemoteStationProvider) DeleteController(controllerType string) error {
	return r.provider.DeleteController(controllerType)
}

func (r *RemoteStationProvider) GetController(controllerType string) (*ControllerData, error) {
	return r.provider.GetController(controllerType)
}

func (r *RemoteStationProvider) IsReadOnly() bool {
	return r.provider.IsReadOnly()
}

func (r *RemoteStationProvider) Close() error {
	return r.provider.Close()
}