package management

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/gorilla/mux"
)

// GetWeatherWebsites returns all configured weather websites
func (h *Handlers) GetWeatherWebsites(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	websites, err := h.controller.ConfigProvider.GetWeatherWebsites()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get weather websites", err)
		return
	}

	response := map[string]interface{}{
		"websites": websites,
		"count":    len(websites),
	}

	h.sendJSON(w, response)
}

// GetWeatherWebsite returns a specific weather website configuration
func (h *Handlers) GetWeatherWebsite(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		h.sendError(w, http.StatusBadRequest, "Website ID is required", nil)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid website ID", err)
		return
	}

	website, err := h.controller.ConfigProvider.GetWeatherWebsite(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Weather website not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to get weather website", err)
		}
		return
	}

	h.sendJSON(w, website)
}

// CreateWeatherWebsite creates a new weather website configuration
func (h *Handlers) CreateWeatherWebsite(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	var website config.WeatherWebsiteData
	if err := json.NewDecoder(r.Body).Decode(&website); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Validate required fields
	if website.Name == "" {
		h.sendError(w, http.StatusBadRequest, "Website name is required", nil)
		return
	}

	// Validate device exists if specified (skip for portal websites)
	if !website.IsPortal && website.DeviceID != nil {
		// Validate that the device ID exists
		devices, err := h.controller.ConfigProvider.GetDevices()
		if err != nil {
			h.sendError(w, http.StatusInternalServerError, "Failed to validate device", err)
			return
		}

		deviceExists := false
		for _, device := range devices {
			if device.ID == *website.DeviceID {
				deviceExists = true
				break
			}
		}

		if !deviceExists {
			h.sendError(w, http.StatusBadRequest, "Specified device does not exist", nil)
			return
		}
	}

	// Validate that regular websites have a device assigned
	if !website.IsPortal && website.DeviceID == nil {
		h.sendError(w, http.StatusBadRequest, "Device is required for regular websites", nil)
		return
	}

	// Add the website
	if err := h.controller.ConfigProvider.AddWeatherWebsite(&website); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.sendError(w, http.StatusConflict, "Weather website already exists", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to create weather website", err)
		}
		return
	}

	// Trigger website configuration reload in the REST controller
	if h.controller.app != nil {
		// Use a type assertion to access the ReloadWebsiteConfiguration method
		type WebsiteReloader interface {
			ReloadWebsiteConfiguration() error
		}

		if websiteReloader, ok := h.controller.app.(WebsiteReloader); ok {
			if err := websiteReloader.ReloadWebsiteConfiguration(); err != nil {
				h.controller.logger.Errorf("Failed to reload website configuration: %v", err)
				// Don't fail the API call - just log the error
			} else {
				h.controller.logger.Info("Website configuration reloaded successfully")
			}
		}
	}

	h.sendJSONWithStatus(w, http.StatusCreated, map[string]interface{}{
		"message": "Weather website created successfully",
		"website": website,
	})
}

// UpdateWeatherWebsite updates an existing weather website configuration
func (h *Handlers) UpdateWeatherWebsite(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		h.sendError(w, http.StatusBadRequest, "Website ID is required", nil)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid website ID", err)
		return
	}

	var website config.WeatherWebsiteData
	if err := json.NewDecoder(r.Body).Decode(&website); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Validate required fields
	if website.Name == "" {
		h.sendError(w, http.StatusBadRequest, "Website name is required", nil)
		return
	}

	// Validate device exists if specified (skip for portal websites)
	if !website.IsPortal && website.DeviceID != nil {
		// Validate that the device ID exists
		devices, err := h.controller.ConfigProvider.GetDevices()
		if err != nil {
			h.sendError(w, http.StatusInternalServerError, "Failed to validate device", err)
			return
		}

		deviceExists := false
		for _, device := range devices {
			if device.ID == *website.DeviceID {
				deviceExists = true
				break
			}
		}

		if !deviceExists {
			h.sendError(w, http.StatusBadRequest, "Specified device does not exist", nil)
			return
		}
	}

	// Validate that regular websites have a device assigned
	if !website.IsPortal && website.DeviceID == nil {
		h.sendError(w, http.StatusBadRequest, "Device is required for regular websites", nil)
		return
	}

	// Update the website
	if err := h.controller.ConfigProvider.UpdateWeatherWebsite(id, &website); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Weather website not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to update weather website", err)
		}
		return
	}

	// Trigger website configuration reload in the REST controller
	if h.controller.app != nil {
		// Use a type assertion to access the ReloadWebsiteConfiguration method
		type WebsiteReloader interface {
			ReloadWebsiteConfiguration() error
		}

		if websiteReloader, ok := h.controller.app.(WebsiteReloader); ok {
			if err := websiteReloader.ReloadWebsiteConfiguration(); err != nil {
				h.controller.logger.Errorf("Failed to reload website configuration: %v", err)
				// Don't fail the API call - just log the error
			} else {
				h.controller.logger.Info("Website configuration reloaded successfully")
			}
		}
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Weather website updated successfully",
		"website": website,
	})
}

// DeleteWeatherWebsite deletes a weather website configuration
func (h *Handlers) DeleteWeatherWebsite(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		h.sendError(w, http.StatusBadRequest, "Website ID is required", nil)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid website ID", err)
		return
	}

	// Delete the website
	if err := h.controller.ConfigProvider.DeleteWeatherWebsite(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Weather website not found", err)
		} else if strings.Contains(err.Error(), "still reference it") {
			h.sendError(w, http.StatusConflict, "Cannot delete website with associated devices", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to delete weather website", err)
		}
		return
	}

	// Trigger website configuration reload in the REST controller
	if h.controller.app != nil {
		// Use a type assertion to access the ReloadWebsiteConfiguration method
		type WebsiteReloader interface {
			ReloadWebsiteConfiguration() error
		}

		if websiteReloader, ok := h.controller.app.(WebsiteReloader); ok {
			if err := websiteReloader.ReloadWebsiteConfiguration(); err != nil {
				h.controller.logger.Errorf("Failed to reload website configuration: %v", err)
				// Don't fail the API call - just log the error
			} else {
				h.controller.logger.Info("Website configuration reloaded successfully")
			}
		}
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Weather website deleted successfully",
	})
}
