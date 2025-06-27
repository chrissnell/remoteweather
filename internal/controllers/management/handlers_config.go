package management

import (
	"net/http"
	"time"
)

// GetWeatherStations returns all configured weather stations
func (h *Handlers) GetWeatherStations(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	devices, err := h.controller.ConfigProvider.GetDevices()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to retrieve weather stations", err)
		return
	}

	response := map[string]interface{}{
		"weather_stations": devices,
		"count":            len(devices),
		"timestamp":        time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// CreateWeatherStation creates a new weather station configuration
func (h *Handlers) CreateWeatherStation(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement weather station creation
	response := map[string]interface{}{
		"message": "Weather station creation not yet implemented",
		"todo":    "Implement ConfigProvider integration for creating weather stations",
	}

	h.sendJSON(w, response)
}

// UpdateWeatherStation updates an existing weather station configuration
func (h *Handlers) UpdateWeatherStation(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement weather station update
	response := map[string]interface{}{
		"message": "Weather station update not yet implemented",
		"todo":    "Implement ConfigProvider integration for updating weather stations",
	}

	h.sendJSON(w, response)
}

// DeleteWeatherStation deletes a weather station configuration
func (h *Handlers) DeleteWeatherStation(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement weather station deletion
	response := map[string]interface{}{
		"message": "Weather station deletion not yet implemented",
		"todo":    "Implement ConfigProvider integration for deleting weather stations",
	}

	h.sendJSON(w, response)
}

// GetStorageConfigs returns all configured storage destinations
func (h *Handlers) GetStorageConfigs(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	storage, err := h.controller.ConfigProvider.GetStorageConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to retrieve storage configuration", err)
		return
	}

	// Convert storage config to a more API-friendly format
	storageConfigs := []map[string]interface{}{}

	if storage.InfluxDB != nil {
		storageConfigs = append(storageConfigs, map[string]interface{}{
			"type":    "influxdb",
			"enabled": true,
			"config":  storage.InfluxDB,
		})
	}

	if storage.TimescaleDB != nil {
		storageConfigs = append(storageConfigs, map[string]interface{}{
			"type":    "timescaledb",
			"enabled": true,
			"config":  storage.TimescaleDB,
		})
	}

	if storage.GRPC != nil {
		storageConfigs = append(storageConfigs, map[string]interface{}{
			"type":    "grpc",
			"enabled": true,
			"config":  storage.GRPC,
		})
	}

	if storage.APRS != nil {
		storageConfigs = append(storageConfigs, map[string]interface{}{
			"type":    "aprs",
			"enabled": true,
			"config":  storage.APRS,
		})
	}

	response := map[string]interface{}{
		"storage_configs": storageConfigs,
		"count":           len(storageConfigs),
		"timestamp":       time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// CreateStorageConfig creates a new storage configuration
func (h *Handlers) CreateStorageConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement storage config creation
	response := map[string]interface{}{
		"message": "Storage config creation not yet implemented",
		"todo":    "Implement ConfigProvider integration for creating storage configs",
	}

	h.sendJSON(w, response)
}

// UpdateStorageConfig updates an existing storage configuration
func (h *Handlers) UpdateStorageConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement storage config update
	response := map[string]interface{}{
		"message": "Storage config update not yet implemented",
		"todo":    "Implement ConfigProvider integration for updating storage configs",
	}

	h.sendJSON(w, response)
}

// DeleteStorageConfig deletes a storage configuration
func (h *Handlers) DeleteStorageConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement storage config deletion
	response := map[string]interface{}{
		"message": "Storage config deletion not yet implemented",
		"todo":    "Implement ConfigProvider integration for deleting storage configs",
	}

	h.sendJSON(w, response)
}
