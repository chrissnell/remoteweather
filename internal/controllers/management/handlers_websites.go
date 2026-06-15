package management

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// RegisterWebsiteRadar registers the website's hostname with the Graywolf maps
// service (recording the non-commercial agreement) and enables radar on success.
func (h *Handlers) RegisterWebsiteRadar(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid website ID", err)
		return
	}

	var body struct {
		AgreeNoncommercial bool `json:"agree_noncommercial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}
	if !body.AgreeNoncommercial {
		h.sendError(w, http.StatusBadRequest, "Non-commercial agreement is required", nil)
		return
	}

	website, err := h.controller.ConfigProvider.GetWeatherWebsite(id)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Weather website not found", err)
		return
	}
	if website.Hostname == "" {
		h.sendError(w, http.StatusBadRequest, "Website must have a hostname before enabling radar", nil)
		return
	}

	token, err := registerRadarToken(website.Hostname)
	if err != nil {
		h.sendError(w, http.StatusBadGateway, "Radar registration failed", err)
		return
	}

	now := time.Now().Unix()
	if err := h.controller.ConfigProvider.SetWebsiteRadarRegistration(id, token, now); err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to save radar registration", err)
		return
	}

	h.reloadWebsiteConfig()
	h.sendJSON(w, map[string]interface{}{
		"registered":          true,
		"radar_registered_at": now,
	})
}

// UnregisterWebsiteRadar disables radar and clears the stored token.
func (h *Handlers) UnregisterWebsiteRadar(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid website ID", err)
		return
	}
	if err := h.controller.ConfigProvider.ClearWebsiteRadarRegistration(id); err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to disable radar", err)
		return
	}
	h.reloadWebsiteConfig()
	h.sendJSON(w, map[string]interface{}{"registered": false})
}

// reloadWebsiteConfig triggers the REST controller to pick up config changes,
// matching the pattern used by Create/UpdateWeatherWebsite.
func (h *Handlers) reloadWebsiteConfig() {
	if h.controller.app == nil {
		return
	}
	type WebsiteReloader interface{ ReloadWebsiteConfiguration() error }
	if reloader, ok := h.controller.app.(WebsiteReloader); ok {
		if err := reloader.ReloadWebsiteConfiguration(); err != nil {
			h.controller.logger.Errorf("Failed to reload website configuration: %v", err)
		}
	}
}
