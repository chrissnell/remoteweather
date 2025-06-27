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

// sendJSON sends a JSON response
func (h *Handlers) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
