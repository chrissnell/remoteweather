package management

import (
	"database/sql"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestDeviceConnectivity tests connectivity to a weather station device
func (h *Handlers) TestDeviceConnectivity(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement device connectivity testing
	// This would test connecting to Davis, Campbell Scientific, etc. devices
	response := map[string]interface{}{
		"message":   "Device connectivity testing not yet implemented",
		"todo":      "Implement device connectivity testing for Davis, Campbell Scientific, etc.",
		"timestamp": time.Now().Unix(),
		"test_results": map[string]interface{}{
			"status":  "not_implemented",
			"devices": []interface{}{},
		},
	}

	h.sendJSON(w, response)
}

// TestDatabaseConnectivity tests connectivity to TimescaleDB database
func (h *Handlers) TestDatabaseConnectivity(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	storage, err := h.controller.ConfigProvider.GetStorageConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to retrieve storage configuration", err)
		return
	}

	testResults := []map[string]interface{}{}

	// Test TimescaleDB if configured
	if storage.TimescaleDB != nil && storage.TimescaleDB.ConnectionString != "" {
		connected := false
		errorMessage := ""

		// Try to connect to TimescaleDB
		db, err := sql.Open("postgres", storage.TimescaleDB.ConnectionString)
		if err != nil {
			errorMessage = err.Error()
		} else {
			defer db.Close()
			if err := db.Ping(); err != nil {
				errorMessage = err.Error()
			} else {
				connected = true
			}
		}

		testResults = append(testResults, map[string]interface{}{
			"database":                   "timescaledb",
			"connected":                  connected,
			"error":                      errorMessage,
			"connection_string_provided": true,
		})
	} else {
		testResults = append(testResults, map[string]interface{}{
			"database":                   "timescaledb",
			"connected":                  false,
			"error":                      "No TimescaleDB configuration found",
			"connection_string_provided": false,
		})
	}

	response := map[string]interface{}{
		"test_results": testResults,
		"timestamp":    time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// TestSerialPortConnectivity tests connectivity to a specific serial port
func (h *Handlers) TestSerialPortConnectivity(w http.ResponseWriter, r *http.Request) {
	// Get port from query parameter
	port := r.URL.Query().Get("port")
	if port == "" {
		h.sendError(w, http.StatusBadRequest, "Missing 'port' query parameter", nil)
		return
	}

	// Test if the port exists and is accessible
	accessible := false
	errorMessage := ""

	// Try to stat the device file (Unix-like systems)
	if _, err := os.Stat(port); err == nil {
		accessible = true
	} else {
		errorMessage = err.Error()
	}

	response := map[string]interface{}{
		"test_results": map[string]interface{}{
			"status":     "completed",
			"port":       port,
			"accessible": accessible,
			"error":      errorMessage,
			"test_type":  "file_access",
		},
		"timestamp": time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// TestAPIConnectivity tests connectivity to external APIs (PWSWeather, Wunderground, etc.)
func (h *Handlers) TestAPIConnectivity(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement external API connectivity testing
	response := map[string]interface{}{
		"message":   "External API connectivity testing not yet implemented",
		"todo":      "Implement connectivity testing for PWSWeather, Wunderground, AerisWeather",
		"timestamp": time.Now().Unix(),
		"test_results": map[string]interface{}{
			"status": "not_implemented",
			"apis": []interface{}{
				map[string]interface{}{
					"name":      "pwsweather",
					"status":    "not_tested",
					"reachable": false,
				},
				map[string]interface{}{
					"name":      "wunderground",
					"status":    "not_tested",
					"reachable": false,
				},
				map[string]interface{}{
					"name":      "aerisweather",
					"status":    "not_tested",
					"reachable": false,
				},
			},
		},
	}

	h.sendJSON(w, response)
}
