package management

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
)

// SnowControllerStatus represents the operational status of the snow controller
type SnowControllerStatus struct {
	ControllerRunning bool       `json:"controller_running"`
	LastCalculation   *time.Time `json:"last_calculation,omitempty"`
	DataAvailable     bool       `json:"data_available"`
	CachedValues      *struct {
		Midnight float64   `json:"midnight"`
		Day24h   float64   `json:"day_24h"`
		Day72h   float64   `json:"day_72h"`
		Season   float64   `json:"season"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	} `json:"cached_values,omitempty"`
	ErrorCount  int    `json:"error_count"`
	LastError   string `json:"last_error,omitempty"`
}

// NOTE: GetSnowConfig and UpdateSnowConfig have been removed.
// Snow controller configuration is now managed via /config/controllers CRUD operations.
// See handlers_controllers.go for validation and sanitization.

// GetSnowStatus returns the operational status of the snow controller
func (h *Handlers) GetSnowStatus(w http.ResponseWriter, r *http.Request) {
	// Get configuration database
	configDB := h.getConfigDB()
	if configDB == nil {
		h.sendError(w, http.StatusServiceUnavailable, "Configuration database not available", nil)
		return
	}

	status := SnowControllerStatus{}

	// Get controller configuration to check if enabled
	var enabled bool
	var lastCalc sql.NullTime
	var errorCount int
	var lastError string

	err := configDB.QueryRow(`
		SELECT enabled, last_calculation_time, error_count, last_error
		FROM snow_controller_settings
		WHERE id = 1
	`).Scan(&enabled, &lastCalc, &errorCount, &lastError)

	if err != nil && err != sql.ErrNoRows {
		h.sendError(w, http.StatusInternalServerError, "Failed to get snow status", err)
		return
	}

	status.ControllerRunning = enabled
	status.ErrorCount = errorCount
	status.LastError = lastError

	if lastCalc.Valid {
		status.LastCalculation = &lastCalc.Time
	}

	// Get weather database connection for cached values
	weatherDB := h.getWeatherDB()
	if weatherDB != nil {
		cachedValues := struct {
			Midnight  float64    `json:"midnight"`
			Day24h    float64    `json:"day_24h"`
			Day72h    float64    `json:"day_72h"`
			Season    float64    `json:"season"`
			UpdatedAt *time.Time `json:"updated_at,omitempty"`
		}{}

		var updatedAt sql.NullTime
		err := weatherDB.QueryRow(`
			SELECT snow_midnight, snow_24h, snow_72h, snow_seasonal, computed_at
			FROM snow_totals_cache
			ORDER BY computed_at DESC
			LIMIT 1
		`).Scan(
			&cachedValues.Midnight,
			&cachedValues.Day24h,
			&cachedValues.Day72h,
			&cachedValues.Season,
			&updatedAt,
		)

		if err == nil {
			if updatedAt.Valid {
				cachedValues.UpdatedAt = &updatedAt.Time
			}
			status.CachedValues = &cachedValues
			status.DataAvailable = true
		} else if err != sql.ErrNoRows {
			h.controller.logger.Warnf("Failed to get cached snow values: %v", err)
		}
	}

	h.sendJSON(w, status)
}

// RecalculateSnow triggers an immediate snow recalculation
func (h *Handlers) RecalculateSnow(w http.ResponseWriter, r *http.Request) {
	// This endpoint would need to signal the snow cache controller to run immediately
	// For now, we'll just return a message indicating that the next scheduled run will happen within 30 seconds
	h.sendJSON(w, map[string]interface{}{
		"message": "Snow calculations run every 30 seconds automatically. Next calculation will occur within 30 seconds.",
	})
}

// getConfigDB gets the SQLite configuration database connection
func (h *Handlers) getConfigDB() *sql.DB {
	provider, ok := h.controller.ConfigProvider.(*config.SQLiteProvider)
	if !ok {
		return nil
	}
	return provider.GetDB()
}

// getWeatherDB gets the TimescaleDB weather database connection
func (h *Handlers) getWeatherDB() *sql.DB {
	// Try to get TimescaleDB connection from storage configuration
	cfg, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil || cfg.Storage.TimescaleDB == nil {
		return nil
	}

	// For now, we'll return nil and log a message
	// In a full implementation, we'd maintain a connection pool
	// or get the connection from the storage manager
	h.controller.logger.Debug("Weather database access not yet implemented in management API")
	return nil
}
