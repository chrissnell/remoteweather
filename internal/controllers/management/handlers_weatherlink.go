package management

import (
	"encoding/json"
	"net/http"

	"github.com/chrissnell/remoteweather/internal/weatherstations/weatherlinklive"
)

// GetWeatherLinkDiscovery queries a WeatherLink Live device and returns active sensors
func (h *Handlers) GetWeatherLinkDiscovery(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		h.sendError(w, http.StatusBadRequest, "Missing required parameter: host", nil)
		return
	}

	// Get current conditions from device
	resp, err := weatherlinklive.GetCurrentConditions(r.Context(), host)
	if err != nil {
		h.sendError(w, http.StatusServiceUnavailable, "Failed to connect to WeatherLink Live device", err)
		return
	}

	// Discover sensors
	discovery := weatherlinklive.DiscoverSensors(resp.Data.Conditions)

	// Add device ID and timestamp from response
	discovery.DID = resp.Data.DID
	discovery.Timestamp = resp.Data.Timestamp

	h.sendJSON(w, discovery)
}

// ValidateWeatherLinkMapping validates a sensor mapping string
func (h *Handlers) ValidateWeatherLinkMapping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MappingString string `json:"mapping_string"`
		Host          string `json:"host,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.MappingString == "" {
		h.sendError(w, http.StatusBadRequest, "Missing required field: mapping_string", nil)
		return
	}

	var validation *weatherlinklive.ValidationResult

	// If host is provided, validate against actual device
	if req.Host != "" {
		// Parse mapping string first
		mappings, err := weatherlinklive.ParseMappingString(req.MappingString)
		if err != nil {
			h.sendError(w, http.StatusBadRequest, "Invalid mapping string", err)
			return
		}

		// Validate against device
		validation = weatherlinklive.ValidateMappings(r.Context(), req.Host, mappings)
	} else {
		// Validate mapping string syntax only
		validation = weatherlinklive.ValidateMappingString(req.MappingString)
	}

	h.sendJSON(w, validation)
}

// GetWeatherLinkTemplates returns available sensor configuration templates
func (h *Handlers) GetWeatherLinkTemplates(w http.ResponseWriter, r *http.Request) {
	templates := weatherlinklive.ListTemplates()

	response := map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}

	h.sendJSON(w, response)
}

// GetWeatherLinkTemplate returns a specific template by ID
func (h *Handlers) GetWeatherLinkTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.URL.Query().Get("id")
	if templateID == "" {
		h.sendError(w, http.StatusBadRequest, "Missing required parameter: id", nil)
		return
	}

	template := weatherlinklive.GetTemplate(templateID)
	if template == nil {
		h.sendError(w, http.StatusNotFound, "Template not found", nil)
		return
	}

	h.sendJSON(w, template)
}

// ValidateWeatherLinkTemplate validates a template configuration
func (h *Handlers) ValidateWeatherLinkTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.URL.Query().Get("id")
	if templateID == "" {
		h.sendError(w, http.StatusBadRequest, "Missing required parameter: id", nil)
		return
	}

	template := weatherlinklive.GetTemplate(templateID)
	if template == nil {
		h.sendError(w, http.StatusNotFound, "Template not found", nil)
		return
	}

	// Validate template mapping string
	err := weatherlinklive.ValidateTemplate(template)
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Template validation failed", err)
		return
	}

	// Return validation result
	validation := weatherlinklive.ValidateMappingString(template.MappingString)

	response := map[string]interface{}{
		"template":   template,
		"validation": validation,
	}

	h.sendJSON(w, response)
}

// SuggestWeatherLinkTemplate suggests a template based on discovered sensors
func (h *Handlers) SuggestWeatherLinkTemplate(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		h.sendError(w, http.StatusBadRequest, "Missing required parameter: host", nil)
		return
	}

	// Get current conditions from device
	resp, err := weatherlinklive.GetCurrentConditions(r.Context(), host)
	if err != nil {
		h.sendError(w, http.StatusServiceUnavailable, "Failed to connect to WeatherLink Live device", err)
		return
	}

	// Suggest template based on discovered sensors
	templateID := weatherlinklive.SuggestTemplate(resp.Data.Conditions)

	response := map[string]interface{}{
		"suggested_template": templateID,
	}

	// If a template is suggested (not custom), include template details
	if templateID != "custom" {
		template := weatherlinklive.GetTemplate(templateID)
		if template != nil {
			response["template"] = template
		}
	}

	h.sendJSON(w, response)
}
