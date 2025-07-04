package management

import (
	"encoding/json"
	"net/http"
	"time"
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
	http.SetCookie(w, &http.Cookie{
		Name:     "rw_session",
		Value:    request.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil, // Only set Secure flag if using HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7, // 7 days
	})

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
		if err == nil && cookie.Value == h.controller.managementConfig.AuthToken {
			authenticated = true
		}
	}

	h.sendJSON(w, map[string]interface{}{
		"authenticated": authenticated,
	})
}
