package management

import (
	"fmt"
	"net/http"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// GetStatus returns the status of the management API
func (h *Handlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"message":   "Management API is running",
	}

	h.sendJSON(w, status)
}

// GetConfig returns the current configuration
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	config, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to load configuration", err)
		return
	}

	response := map[string]interface{}{
		"config":    config,
		"timestamp": time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// ValidateConfig validates the current configuration and returns any errors
func (h *Handlers) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	configData, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to load configuration", err)
		return
	}

	validationErrors := config.ValidateConfig(configData)

	response := map[string]interface{}{
		"valid":     len(validationErrors) == 0,
		"errors":    validationErrors,
		"timestamp": time.Now().Unix(),
	}

	if len(validationErrors) > 0 {
		response["message"] = fmt.Sprintf("Configuration has %d validation error(s)", len(validationErrors))
	} else {
		response["message"] = "Configuration is valid"
	}

	h.sendJSON(w, response)
}

// ReloadConfig reloads the configuration dynamically
func (h *Handlers) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.app == nil {
		h.sendError(w, http.StatusServiceUnavailable, "Configuration reload not available", nil)
		return
	}

	h.controller.logger.Info("Management API triggered configuration reload")

	if err := h.controller.app.ReloadConfiguration(h.controller.ctx); err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to reload configuration", err)
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"message":   "Configuration reloaded successfully",
		"timestamp": time.Now().Unix(),
	}

	h.sendJSON(w, response)
}
