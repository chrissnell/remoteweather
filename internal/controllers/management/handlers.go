package management

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// Handlers contains the HTTP handlers for the management API
type Handlers struct {
	controller *Controller
}

// NewHandlers creates a new Handlers instance
func NewHandlers(ctrl *Controller) *Handlers {
	return &Handlers{
		controller: ctrl,
	}
}

// sendJSON sends a JSON response with optional status code
func (h *Handlers) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendJSONWithStatus sends a JSON response with a specific status code
func (h *Handlers) sendJSONWithStatus(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response in JSON format
func (h *Handlers) sendError(w http.ResponseWriter, statusCode int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().Unix(),
	}

	if err != nil {
		errorResponse["details"] = err.Error()
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// GetLogs handles REST API requests for log entries
func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	logBuffer := log.GetLogBuffer()

	// Get all logs and clear the buffer to avoid duplicates on next call
	logs := logBuffer.GetLogs(true) // true = clear buffer

	// Convert to API format
	var logEntries []map[string]interface{}
	for _, entry := range logs {
		logEntry := map[string]interface{}{
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"level":     entry.Level,
			"message":   entry.Message,
		}

		// Add caller if available
		if entry.Caller != "" {
			logEntry["caller"] = entry.Caller
		}

		// Add any additional fields
		if len(entry.Fields) > 0 {
			for key, value := range entry.Fields {
				logEntry[key] = value
			}
		}

		logEntries = append(logEntries, logEntry)
	}

	response := map[string]interface{}{
		"logs":      logEntries,
		"count":     len(logEntries),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	h.sendJSON(w, response)
}

// GetHTTPLogs handles REST API requests for HTTP log entries from the REST server
func (h *Handlers) GetHTTPLogs(w http.ResponseWriter, r *http.Request) {
	httpLogBuffer := log.GetHTTPLogBuffer()

	// Get all logs and clear the buffer to avoid duplicates on next call
	logs := httpLogBuffer.GetLogs(true) // true = clear buffer

	// Convert to API format
	var logEntries []map[string]interface{}
	for _, entry := range logs {
		logEntry := map[string]interface{}{
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"level":     entry.Level,
			"message":   entry.Message,
		}

		// Add caller if available
		if entry.Caller != "" {
			logEntry["caller"] = entry.Caller
		}

		// Add any additional fields
		if len(entry.Fields) > 0 {
			for key, value := range entry.Fields {
				logEntry[key] = value
			}
		}

		logEntries = append(logEntries, logEntry)
	}

	response := map[string]interface{}{
		"logs":      logEntries,
		"count":     len(logEntries),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	h.sendJSON(w, response)
}

// Login handles the login request and sets a session cookie
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if request.Token == "" {
		h.sendError(w, http.StatusBadRequest, "Token is required", nil)
		return
	}

	// Validate the token
	if request.Token != h.controller.managementConfig.AuthToken {
		h.sendError(w, http.StatusUnauthorized, "Invalid token", nil)
		return
	}

	// Set session cookie
	cookie := &http.Cookie{
		Name:     "rw_session",
		Value:    request.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil, // Only set Secure flag if using HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7, // 7 days
	}

	log.Debugf("Setting session cookie: %+v", cookie)
	http.SetCookie(w, cookie)

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "Login successful",
	})
}

// Logout handles the logout request and clears the session cookie
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "rw_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Expire immediately
	})

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "Logout successful",
	})
}

// GetAuthStatus checks if the current session is authenticated
func (h *Handlers) GetAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false

	// Check for Bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		expectedAuth := "Bearer " + h.controller.managementConfig.AuthToken
		if authHeader == expectedAuth {
			authenticated = true
		}
	}

	// Check for session cookie
	if !authenticated {
		cookie, err := r.Cookie("rw_session")
		if err == nil {
			if cookie.Value == h.controller.managementConfig.AuthToken {
				authenticated = true
			} else {
				// Debug: log token mismatch
				log.Debugf("Auth token mismatch - cookie: %s, expected: %s", cookie.Value, h.controller.managementConfig.AuthToken)
			}
		} else {
			// Debug: log missing cookie
			log.Debugf("No session cookie found: %v", err)
		}
	}

	h.sendJSON(w, map[string]interface{}{
		"authenticated": authenticated,
	})
}

// ChangeAdminToken changes the administrator token
func (h *Handlers) ChangeAdminToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		NewToken string `json:"new_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if request.NewToken == "" {
		h.sendError(w, http.StatusBadRequest, "New token is required", nil)
		return
	}

	// Validate new token
	if len(request.NewToken) < 8 {
		h.sendError(w, http.StatusBadRequest, "New token must be at least 8 characters long", nil)
		return
	}

	if request.NewToken == h.controller.managementConfig.AuthToken {
		h.sendError(w, http.StatusBadRequest, "New token must be different from current token", nil)
		return
	}

	// Update the token in the database
	if h.controller.ConfigProvider != nil {
		// Get current management config
		controllers, err := h.controller.ConfigProvider.GetControllers()
		if err != nil {
			h.sendError(w, http.StatusInternalServerError, "Failed to get current configuration", err)
			return
		}

		// Find management controller
		var managementController *config.ControllerData
		for _, controller := range controllers {
			if controller.Type == "management" {
				managementController = &controller
				break
			}
		}

		if managementController == nil {
			h.sendError(w, http.StatusInternalServerError, "Management controller configuration not found", nil)
			return
		}

		// Update the token
		if managementController.ManagementAPI != nil {
			managementController.ManagementAPI.AuthToken = request.NewToken
		}

		// Save updated config
		err = h.controller.ConfigProvider.UpdateController("management", managementController)
		if err != nil {
			h.sendError(w, http.StatusInternalServerError, "Failed to save new token", err)
			return
		}

		// Update the in-memory config
		h.controller.managementConfig.AuthToken = request.NewToken

		h.controller.logger.Info("Administrator token changed successfully")
		h.controller.logger.Info("═══════════════════════════════════════════════════════════════")
		h.controller.logger.Info("           ADMINISTRATOR TOKEN UPDATED                         ")
		h.controller.logger.Info("═══════════════════════════════════════════════════════════════")
		h.controller.logger.Infof("   New Token: %s", request.NewToken)
		h.controller.logger.Info("   All sessions will be invalidated")
		h.controller.logger.Info("═══════════════════════════════════════════════════════════════")
	} else {
		h.sendError(w, http.StatusServiceUnavailable, "Configuration provider not available", nil)
		return
	}

	// Clear the current session cookie to force re-login
	http.SetCookie(w, &http.Cookie{
		Name:     "rw_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Expire immediately
	})

	h.sendJSON(w, map[string]interface{}{
		"success": true,
		"message": "Administrator token changed successfully. Please login again.",
	})
}
