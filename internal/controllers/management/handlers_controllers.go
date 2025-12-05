package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// GetControllers returns all configured controllers (sanitized for security)
func (h *Handlers) GetControllers(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	controllers, err := h.controller.ConfigProvider.GetControllers()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get controllers", err)
		return
	}

	// Convert to a map for easier API consumption, sanitizing sensitive data
	// Filter out weather service controllers since they're now auto-started and device-specific
	weatherServices := map[string]bool{
		"aerisweather": true,
		"pwsweather": true,
		"weatherunderground": true,
		"aprs": true,
	}
	
	controllerMap := make(map[string]interface{})
	for _, controller := range controllers {
		// Skip weather service controllers from display
		if weatherServices[controller.Type] {
			continue
		}
		sanitized := h.sanitizeControllerConfig(&controller)
		controllerMap[controller.Type] = sanitized
	}

	h.sendJSON(w, map[string]interface{}{
		"controllers": controllerMap,
		"count":       len(controllerMap),
	})
}

// GetController returns a specific controller configuration
func (h *Handlers) GetController(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	controllerType := strings.TrimPrefix(r.URL.Path, "/api/config/controllers/")
	if controllerType == "" {
		h.sendError(w, http.StatusBadRequest, "Controller type is required", nil)
		return
	}

	controller, err := h.controller.ConfigProvider.GetController(controllerType)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Controller not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to get controller", err)
		}
		return
	}

	sanitized := h.sanitizeControllerConfig(controller)
	h.sendJSON(w, sanitized)
}

// CreateController creates a new controller configuration
func (h *Handlers) CreateController(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Controller type is required", nil)
		return
	}

	// Weather service controllers are auto-started and device-specific, cannot be created via API
	weatherServices := map[string]bool{
		"aerisweather": true,
		"pwsweather": true,
		"weatherunderground": true,
		"aprs": true,
	}
	
	if weatherServices[requestData.Type] {
		h.sendError(w, http.StatusBadRequest, "Weather service controllers are automatically managed and cannot be created manually", nil)
		return
	}

	// Validate controller type
	validTypes := []string{"rest", "management", "snowcache"}
	if !contains(validTypes, requestData.Type) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid controller type. Must be one of: %v", validTypes), nil)
		return
	}

	// Convert config to appropriate type
	controllerConfig, err := h.convertControllerConfig(requestData.Type, requestData.Config)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid controller configuration", err)
		return
	}

	// Validate controller configuration
	if err := h.validateControllerConfig(requestData.Type, controllerConfig); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid controller configuration", err)
		return
	}

	// Add the controller config
	if err := h.controller.ConfigProvider.AddController(controllerConfig); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.sendError(w, http.StatusConflict, "Controller configuration already exists", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to create controller config", err)
		}
		return
	}

	// Dynamically start the controller
	if h.controller.app != nil {
		if err := h.controller.app.AddController(controllerConfig); err != nil {
			h.controller.logger.Errorf("Failed to start controller %s: %v", requestData.Type, err)
			// Don't fail the API call since the config was saved successfully
			// The controller will start on next app restart
		} else {
			h.controller.logger.Infof("Controller %s started successfully", requestData.Type)
		}
	}

	h.sendJSONWithStatus(w, http.StatusCreated, map[string]interface{}{
		"message": "Controller configuration created successfully",
		"type":    requestData.Type,
		"config":  h.sanitizeControllerConfig(controllerConfig),
	})
}

// UpdateController updates an existing controller configuration
func (h *Handlers) UpdateController(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	controllerType := strings.TrimPrefix(r.URL.Path, "/api/config/controllers/")
	if controllerType == "" {
		h.sendError(w, http.StatusBadRequest, "Controller type is required", nil)
		return
	}

	// Weather service controllers are auto-started and device-specific, cannot be updated via API
	weatherServices := map[string]bool{
		"aerisweather": true,
		"pwsweather": true,
		"weatherunderground": true,
		"aprs": true,
	}
	
	if weatherServices[controllerType] {
		h.sendError(w, http.StatusBadRequest, "Weather service controllers are automatically managed and cannot be updated manually", nil)
		return
	}

	// Validate controller type
	validTypes := []string{"rest", "management", "snowcache"}
	if !contains(validTypes, controllerType) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid controller type. Must be one of: %v", validTypes), nil)
		return
	}

	var configData interface{}
	if err := json.NewDecoder(r.Body).Decode(&configData); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON", err)
		return
	}

	// Convert config to appropriate type
	controllerConfig, err := h.convertControllerConfig(controllerType, configData)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid controller configuration", err)
		return
	}

	// Validate controller configuration
	if err := h.validateControllerConfig(controllerType, controllerConfig); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid controller configuration", err)
		return
	}

	// Update the controller config
	if err := h.controller.ConfigProvider.UpdateController(controllerType, controllerConfig); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Controller configuration not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to update controller config", err)
		}
		return
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Controller configuration updated successfully",
		"type":    controllerType,
		"config":  h.sanitizeControllerConfig(controllerConfig),
	})
}

// DeleteController deletes a controller configuration
func (h *Handlers) DeleteController(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider.IsReadOnly() {
		h.sendError(w, http.StatusBadRequest, "Configuration is read-only", nil)
		return
	}

	controllerType := strings.TrimPrefix(r.URL.Path, "/api/config/controllers/")
	if controllerType == "" {
		h.sendError(w, http.StatusBadRequest, "Controller type is required", nil)
		return
	}

	// Weather service controllers are auto-started and device-specific, cannot be deleted via API
	weatherServices := map[string]bool{
		"aerisweather": true,
		"pwsweather": true,
		"weatherunderground": true,
		"aprs": true,
	}
	
	if weatherServices[controllerType] {
		h.sendError(w, http.StatusBadRequest, "Weather service controllers are automatically managed and cannot be deleted manually", nil)
		return
	}

	// Validate controller type
	validTypes := []string{"rest", "management", "snowcache"}
	if !contains(validTypes, controllerType) {
		h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid controller type. Must be one of: %v", validTypes), nil)
		return
	}

	// Delete the controller config
	if err := h.controller.ConfigProvider.DeleteController(controllerType); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.sendError(w, http.StatusNotFound, "Controller configuration not found", err)
		} else {
			h.sendError(w, http.StatusInternalServerError, "Failed to delete controller config", err)
		}
		return
	}

	// Dynamically stop the controller
	if h.controller.app != nil {
		if err := h.controller.app.RemoveController(controllerType); err != nil {
			h.controller.logger.Errorf("Failed to stop controller %s: %v", controllerType, err)
			// Don't fail the API call since the config was deleted successfully
			// The controller will stop on next app restart
		} else {
			h.controller.logger.Infof("Controller %s stopped successfully", controllerType)
		}
	}

	h.sendJSON(w, map[string]interface{}{
		"message": "Controller configuration deleted successfully",
	})
}

// Helper functions for validation and conversion

// sanitizeControllerConfig removes sensitive information from controller config for API responses
func (h *Handlers) sanitizeControllerConfig(controller *config.ControllerData) map[string]interface{} {
	sanitized := map[string]interface{}{
		"type": controller.Type,
	}

	switch controller.Type {
	case "pwsweather":
		if controller.PWSWeather != nil {
			sanitized["config"] = map[string]interface{}{
				"station_id":       controller.PWSWeather.StationID,
				"api_key":          "[HIDDEN]",
				"api_endpoint":     controller.PWSWeather.APIEndpoint,
				"upload_interval":  controller.PWSWeather.UploadInterval,
				"pull_from_device": controller.PWSWeather.PullFromDevice,
			}
		}
	case "weatherunderground":
		if controller.WeatherUnderground != nil {
			sanitized["config"] = map[string]interface{}{
				"station_id":       controller.WeatherUnderground.StationID,
				"api_key":          "[HIDDEN]",
				"api_endpoint":     controller.WeatherUnderground.APIEndpoint,
				"upload_interval":  controller.WeatherUnderground.UploadInterval,
				"pull_from_device": controller.WeatherUnderground.PullFromDevice,
			}
		}
	case "aerisweather":
		if controller.AerisWeather != nil {
			sanitized["config"] = map[string]interface{}{
				"api_client_id":     "[HIDDEN]",
				"api_client_secret": "[HIDDEN]",
				"api_endpoint":      controller.AerisWeather.APIEndpoint,
				"latitude":          controller.AerisWeather.Latitude,
				"longitude":         controller.AerisWeather.Longitude,
			}
		}
	case "rest":
		if controller.RESTServer != nil {
			config := map[string]interface{}{
				"http_port":           controller.RESTServer.HTTPPort,
				"default_listen_addr": controller.RESTServer.DefaultListenAddr,
			}
			if controller.RESTServer.HTTPSPort != nil {
				config["https_port"] = *controller.RESTServer.HTTPSPort
			}
			if controller.RESTServer.GRPCPort > 0 {
				config["grpc_port"] = controller.RESTServer.GRPCPort
			}
			if controller.RESTServer.GRPCListenAddr != "" {
				config["grpc_listen_addr"] = controller.RESTServer.GRPCListenAddr
			}
			sanitized["config"] = config
		}
	case "management":
		if controller.ManagementAPI != nil {
			sanitized["config"] = map[string]interface{}{
				"port":        controller.ManagementAPI.Port,
				"listen_addr": controller.ManagementAPI.ListenAddr,
				"auth_token":  "[CONFIGURED]",
				"cert":        h.getConfiguredStatus(controller.ManagementAPI.Cert),
				"key":         h.getConfiguredStatus(controller.ManagementAPI.Key),
			}
		}
	case "aprs":
		if controller.APRS != nil {
			config := map[string]interface{}{
				"server": controller.APRS.Server,
			}
			if controller.APRS.Health != nil {
				config["health"] = controller.APRS.Health
			}
			sanitized["config"] = config
		}
	case "snowcache":
		if controller.SnowCache != nil {
			sanitized["config"] = map[string]interface{}{
				"station_name":     controller.SnowCache.StationName,
				"base_distance":    controller.SnowCache.BaseDistance,
				"smoothing_window": controller.SnowCache.SmoothingWindow,
				"penalty":          controller.SnowCache.Penalty,
				"min_accumulation": controller.SnowCache.MinAccumulation,
				"min_segment_size": controller.SnowCache.MinSegmentSize,
			}
		}
	}

	return sanitized
}

// getConfiguredStatus returns "[CONFIGURED]" if value is non-empty, empty string otherwise
func (h *Handlers) getConfiguredStatus(value string) string {
	if value != "" {
		return "[CONFIGURED]"
	}
	return ""
}

// convertControllerConfig converts interface{} to appropriate controller config type
func (h *Handlers) convertControllerConfig(controllerType string, configData interface{}) (*config.ControllerData, error) {
	controller := &config.ControllerData{
		Type: controllerType,
	}

	// Convert to JSON and back to get proper types
	jsonData, err := json.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	switch controllerType {
	case "pwsweather":
		var pwsConfig config.PWSWeatherData
		if err := json.Unmarshal(jsonData, &pwsConfig); err != nil {
			return nil, fmt.Errorf("invalid PWS Weather config: %w", err)
		}
		controller.PWSWeather = &pwsConfig

	case "weatherunderground":
		var wuConfig config.WeatherUndergroundData
		if err := json.Unmarshal(jsonData, &wuConfig); err != nil {
			return nil, fmt.Errorf("invalid Weather Underground config: %w", err)
		}
		controller.WeatherUnderground = &wuConfig

	case "aerisweather":
		var aerisConfig config.AerisWeatherData
		if err := json.Unmarshal(jsonData, &aerisConfig); err != nil {
			return nil, fmt.Errorf("invalid Aeris Weather config: %w", err)
		}
		controller.AerisWeather = &aerisConfig

	case "rest":
		var restConfig config.RESTServerData
		if err := json.Unmarshal(jsonData, &restConfig); err != nil {
			return nil, fmt.Errorf("invalid REST server config: %w", err)
		}
		controller.RESTServer = &restConfig

	case "management":
		var mgmtConfig config.ManagementAPIData
		if err := json.Unmarshal(jsonData, &mgmtConfig); err != nil {
			return nil, fmt.Errorf("invalid Management API config: %w", err)
		}
		controller.ManagementAPI = &mgmtConfig

	case "aprs":
		var aprsConfig config.APRSData
		if err := json.Unmarshal(jsonData, &aprsConfig); err != nil {
			return nil, fmt.Errorf("invalid APRS config: %w", err)
		}
		controller.APRS = &aprsConfig

	case "snowcache":
		var snowConfig config.SnowCacheData
		if err := json.Unmarshal(jsonData, &snowConfig); err != nil {
			return nil, fmt.Errorf("invalid Snow Cache config: %w", err)
		}
		controller.SnowCache = &snowConfig

	default:
		return nil, fmt.Errorf("unsupported controller type: %s", controllerType)
	}

	return controller, nil
}

// validateControllerConfig validates controller-specific configuration
func (h *Handlers) validateControllerConfig(controllerType string, controller *config.ControllerData) error {
	switch controllerType {
	case "pwsweather":
		if controller.PWSWeather == nil {
			return fmt.Errorf("PWS Weather configuration is required")
		}
		if controller.PWSWeather.StationID == "" {
			return fmt.Errorf("PWS Weather station_id is required")
		}
		if controller.PWSWeather.APIKey == "" {
			return fmt.Errorf("PWS Weather api_key is required")
		}

	case "weatherunderground":
		if controller.WeatherUnderground == nil {
			return fmt.Errorf("Weather Underground configuration is required")
		}
		if controller.WeatherUnderground.StationID == "" {
			return fmt.Errorf("Weather Underground station_id is required")
		}
		if controller.WeatherUnderground.APIKey == "" {
			return fmt.Errorf("Weather Underground api_key is required")
		}

	case "aerisweather":
		if controller.AerisWeather == nil {
			return fmt.Errorf("Aeris Weather configuration is required")
		}
		if controller.AerisWeather.APIClientID == "" {
			return fmt.Errorf("Aeris Weather api_client_id is required")
		}
		if controller.AerisWeather.APIClientSecret == "" {
			return fmt.Errorf("Aeris Weather api_client_secret is required")
		}

	case "rest":
		if controller.RESTServer == nil {
			return fmt.Errorf("REST Server configuration is required")
		}
		if controller.RESTServer.HTTPPort <= 0 || controller.RESTServer.HTTPPort > 65535 {
			return fmt.Errorf("REST Server HTTP port must be between 1 and 65535")
		}

		// Validate gRPC port if provided
		if controller.RESTServer.GRPCPort > 0 {
			if controller.RESTServer.GRPCPort > 65535 {
				return fmt.Errorf("gRPC port must be between 1 and 65535")
			}

			// Check for port conflict with REST HTTP port
			if controller.RESTServer.GRPCPort == controller.RESTServer.HTTPPort {
				return fmt.Errorf("gRPC port cannot be the same as HTTP port")
			}

			// Check for port conflict with REST HTTPS port
			if controller.RESTServer.HTTPSPort != nil && controller.RESTServer.GRPCPort == *controller.RESTServer.HTTPSPort {
				return fmt.Errorf("gRPC port cannot be the same as HTTPS port")
			}
		}

		// Validate gRPC TLS config - both cert and key must be provided together
		grpcCertProvided := controller.RESTServer.GRPCCertPath != ""
		grpcKeyProvided := controller.RESTServer.GRPCKeyPath != ""

		if grpcCertProvided != grpcKeyProvided {
			return fmt.Errorf("gRPC cert and key must both be provided or both be empty")
		}

		// REST Server can now start without weather websites configured
		// The server will return appropriate error messages for weather endpoints
		// when no websites are configured

	case "management":
		if controller.ManagementAPI == nil {
			return fmt.Errorf("Management API configuration is required")
		}
		if controller.ManagementAPI.Port <= 0 || controller.ManagementAPI.Port > 65535 {
			return fmt.Errorf("Management API port must be between 1 and 65535")
		}

	case "aprs":
		if controller.APRS == nil {
			return fmt.Errorf("APRS configuration is required")
		}
		if controller.APRS.Server == "" {
			return fmt.Errorf("APRS server is required")
		}

		// Check if at least one device has APRS enabled with callsign and location
		devices, err := h.controller.ConfigProvider.GetDevices()
		if err != nil {
			return fmt.Errorf("failed to check device configurations: %w", err)
		}

		validStation := false
		for _, device := range devices {
			if device.APRSEnabled && device.APRSCallsign != "" &&
				device.Latitude != 0 && device.Longitude != 0 {
				validStation = true
				break
			}
		}

		if !validStation {
			return fmt.Errorf("APRS controller requires at least one weather station to have APRS enabled with a callsign and location configured. Please enable APRS on a weather station first.")
		}

	case "snowcache":
		if controller.SnowCache == nil {
			return fmt.Errorf("Snow Cache configuration is required")
		}
		if controller.SnowCache.StationName == "" {
			return fmt.Errorf("Snow Cache station_name is required")
		}
		// Verify the station exists
		_, err := h.controller.ConfigProvider.GetDevice(controller.SnowCache.StationName)
		if err != nil {
			return fmt.Errorf("Snow Cache station not found: %w", err)
		}
		if controller.SnowCache.BaseDistance <= 0 {
			return fmt.Errorf("Snow Cache base_distance must be greater than 0")
		}
		if controller.SnowCache.SmoothingWindow < 1 || controller.SnowCache.SmoothingWindow > 24 {
			return fmt.Errorf("Snow Cache smoothing_window must be between 1 and 24")
		}
		if controller.SnowCache.Penalty <= 0 {
			return fmt.Errorf("Snow Cache penalty must be greater than 0")
		}
		if controller.SnowCache.MinAccumulation < 0 {
			return fmt.Errorf("Snow Cache min_accumulation cannot be negative")
		}
		if controller.SnowCache.MinSegmentSize < 1 {
			return fmt.Errorf("Snow Cache min_segment_size must be at least 1")
		}
	}

	return nil
}
