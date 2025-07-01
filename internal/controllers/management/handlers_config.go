package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// GetWeatherStations returns all configured weather stations
func (h *Handlers) GetWeatherStations(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	devices, err := h.controller.ConfigProvider.GetDevices()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get devices", err)
		return
	}

	response := map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	}

	h.sendJSON(w, response)
}

// GetWeatherStation returns a specific weather station configuration
func (h *Handlers) GetWeatherStation(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	deviceName := strings.TrimPrefix(r.URL.Path, "/api/config/weather-stations/")
	if deviceName == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}

	device, err := h.controller.ConfigProvider.GetDevice(deviceName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Device not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to get device", err)
		}
		return
	}

	h.sendJSON(w, device)
}

// CreateWeatherStation creates a new weather station configuration
func (h *Handlers) CreateWeatherStation(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	var device config.DeviceData
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Validate required fields
	if device.Name == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}
	if device.Type == "" {
		h.sendError(w, http.StatusBadRequest, "Device type is required", nil)
		return
	}

	// Validate device type
	validTypes := []string{"campbellscientific", "davis", "snowgauge"}
	if !contains(validTypes, device.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid device type. Must be one of: %v", validTypes), nil)
		return
	}

	// Validate connection settings based on type
	if err := h.validateDeviceConnectionSettings(&device); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid device configuration", err)
		return
	}

	// Add the device
	if err := h.controller.ConfigProvider.AddDevice(&device); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.sendError(w, http.StatusConflict, "Device already exists", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to create device", err)
		}
		return
	}

	h.sendJSONWithStatus(w, http.StatusCreated, map[string]interface{}{
		"message": "Weather station created successfully",
		"device":  device,
	})
}

// UpdateWeatherStation updates an existing weather station configuration
func (h *Handlers) UpdateWeatherStation(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	deviceName := strings.TrimPrefix(r.URL.Path, "/api/config/weather-stations/")
	if deviceName == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}

	var device config.DeviceData
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Validate required fields
	if device.Name == "" {
		device.Name = deviceName // Use URL parameter if not provided in body
	}
	if device.Type == "" {
		h.sendError(w, http.StatusBadRequest, "Device type is required", nil)
		return
	}

	// Validate device type
	validTypes := []string{"campbellscientific", "davis", "snowgauge"}
	if !contains(validTypes, device.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid device type. Must be one of: %v", validTypes), nil)
		return
	}

	// Validate connection settings
	if err := h.validateDeviceConnectionSettings(&device); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid device configuration", err)
		return
	}

	// Update the device
	if err := h.controller.ConfigProvider.UpdateDevice(deviceName, &device); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Device not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to update device", err)
		}
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Weather station updated successfully",
		"device":  device,
	})
}

// DeleteWeatherStation deletes a weather station configuration
func (h *Handlers) DeleteWeatherStation(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	deviceName := strings.TrimPrefix(r.URL.Path, "/api/config/weather-stations/")
	if deviceName == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}

	// Delete the device
	if err := h.controller.ConfigProvider.DeleteDevice(deviceName); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Device not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to delete device", err)
		}
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Weather station deleted successfully",
	})
}

// GetStorageConfigs returns all configured storage backends
func (h *Handlers) GetStorageConfigs(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	storageConfig, err := h.controller.ConfigProvider.GetStorageConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get storage config", err)
		return
	}

	// Convert to a map for easier API consumption
	storageMap := make(map[string]interface{})
	if storageConfig.TimescaleDB != nil {
		storageMap["timescaledb"] = storageConfig.TimescaleDB
	}
	if storageConfig.GRPC != nil {
		storageMap["grpc"] = storageConfig.GRPC
	}

	h.sendJSON(w, map[string]interface{}{
		"storage": storageMap,
		"count":   len(storageMap),
	})
}

// CreateStorageConfig creates a new storage backend configuration
func (h *Handlers) CreateStorageConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	var requestData struct {
		Type   string      `json:"type"`
		Config interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	if requestData.Type == "" {
		h.sendError(w, http.StatusBadRequest, "Storage type is required", nil)
		return
	}

	// Validate storage type
	validTypes := []string{"timescaledb", "grpc"}
	if !contains(validTypes, requestData.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid storage type. Must be one of: %v", validTypes), nil)
		return
	}

	// Convert config to appropriate type
	storageConfig, err := h.convertStorageConfig(requestData.Type, requestData.Config)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid storage configuration", err)
		return
	}

	// Validate storage configuration
	if err := h.validateStorageConfig(requestData.Type, storageConfig); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid storage configuration", err)
		return
	}

	// Add the storage config
	if err := h.controller.ConfigProvider.AddStorageConfig(requestData.Type, storageConfig); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.sendError(w, http.StatusConflict, "Storage configuration already exists", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to create storage config", err)
		}
		return
	}

	h.sendJSONWithStatus(w, http.StatusCreated, map[string]interface{}{
		"message": "Storage configuration created successfully",
		"type":    requestData.Type,
		"config":  storageConfig,
	})
}

// UpdateStorageConfig updates an existing storage backend configuration
func (h *Handlers) UpdateStorageConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	storageType := strings.TrimPrefix(r.URL.Path, "/api/config/storage/")
	if storageType == "" {
		h.sendError(w, http.StatusBadRequest, "Storage type is required", nil)
		return
	}

	// Validate storage type
	validTypes := []string{"timescaledb", "grpc"}
	if !contains(validTypes, storageType) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid storage type. Must be one of: %v", validTypes), nil)
		return
	}

	var configData interface{}
	if err := json.NewDecoder(r.Body).Decode(&configData); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Convert config to appropriate type
	storageConfig, err := h.convertStorageConfig(storageType, configData)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid storage configuration", err)
		return
	}

	// Validate storage configuration
	if err := h.validateStorageConfig(storageType, storageConfig); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid storage configuration", err)
		return
	}

	// Update the storage config
	if err := h.controller.ConfigProvider.UpdateStorageConfig(storageType, storageConfig); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Storage configuration not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to update storage config", err)
		}
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Storage configuration updated successfully",
		"type":    storageType,
		"config":  storageConfig,
	})
}

// DeleteStorageConfig deletes a storage backend configuration
func (h *Handlers) DeleteStorageConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	storageType := strings.TrimPrefix(r.URL.Path, "/api/config/storage/")
	if storageType == "" {
		h.sendError(w, http.StatusBadRequest, "Storage type is required", nil)
		return
	}

	// Validate storage type
	validTypes := []string{"timescaledb", "grpc"}
	if !contains(validTypes, storageType) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid storage type. Must be one of: %v", validTypes), nil)
		return
	}

	// Delete the storage config
	if err := h.controller.ConfigProvider.DeleteStorageConfig(storageType); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Storage configuration not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to delete storage config", err)
		}
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Storage configuration deleted successfully",
	})
}

// Helper functions for validation and conversion

// validateDeviceConnectionSettings validates device connection settings based on type
func (h *Handlers) validateDeviceConnectionSettings(device *config.DeviceData) error {
	switch device.Type {
	case "campbellscientific", "davis":
		// These devices support both TCP and serial connections
		if device.Hostname != "" && device.Port != "" {
			// TCP connection - validate hostname and port
			if device.Hostname == "" {
				return fmt.Errorf("hostname is required for TCP connection")
			}
			if device.Port == "" {
				return fmt.Errorf("port is required for TCP connection")
			}
		} else if device.SerialDevice != "" {
			// Serial connection - validate serial device and baud rate
			if device.Baud <= 0 {
				return fmt.Errorf("baud rate is required for serial connection")
			}
			// Validate common baud rates
			validBauds := []int{9600, 19200, 38400, 57600, 115200}
			validBaud := false
			for _, b := range validBauds {
				if device.Baud == b {
					validBaud = true
					break
				}
			}
			if !validBaud {
				return fmt.Errorf("invalid baud rate. Must be one of: %v", validBauds)
			}
		} else {
			return fmt.Errorf("either hostname/port or serial_device must be specified")
		}
	case "snowgauge":
		// Snow gauge uses TCP/IP connection (gRPC)
		if device.Hostname == "" {
			return fmt.Errorf("hostname is required for snow gauge")
		}
		if device.Port == "" {
			return fmt.Errorf("port is required for snow gauge")
		}
		if device.BaseSnowDistance <= 0 {
			return fmt.Errorf("base_snow_distance is required for snow gauge")
		}
	default:
		return fmt.Errorf("unsupported device type: %s", device.Type)
	}

	return nil
}

// convertStorageConfig converts interface{} to appropriate storage config type
func (h *Handlers) convertStorageConfig(storageType string, configData interface{}) (interface{}, error) {
	// Convert to JSON and back to get proper type conversion
	jsonData, err := json.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	switch storageType {
	case "timescaledb":
		var timescaleConfig config.TimescaleDBData
		if err := json.Unmarshal(jsonData, &timescaleConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TimescaleDB config: %w", err)
		}
		return &timescaleConfig, nil
	case "grpc":
		var grpcConfig config.GRPCData
		if err := json.Unmarshal(jsonData, &grpcConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal GRPC config: %w", err)
		}
		return &grpcConfig, nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// validateStorageConfig validates storage configuration based on type
func (h *Handlers) validateStorageConfig(storageType string, configData interface{}) error {
	switch storageType {
	case "timescaledb":
		timescale, ok := configData.(*config.TimescaleDBData)
		if !ok {
			return fmt.Errorf("invalid TimescaleDB config type")
		}
		if timescale.ConnectionString == "" {
			return fmt.Errorf("connection_string is required for TimescaleDB")
		}
	case "grpc":
		grpc, ok := configData.(*config.GRPCData)
		if !ok {
			return fmt.Errorf("invalid GRPC config type")
		}
		if grpc.Port <= 0 {
			return fmt.Errorf("port is required for GRPC")
		}
		if grpc.PullFromDevice == "" {
			return fmt.Errorf("pull_from_device is required for GRPC")
		}
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
