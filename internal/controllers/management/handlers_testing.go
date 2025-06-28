package management

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	serial "github.com/tarm/goserial"
)

// TestDeviceConnectivity tests connectivity to a specific weather station device
func (h *Handlers) TestDeviceConnectivity(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceName string `json:"device_name"`
		Timeout    int    `json:"timeout,omitempty"` // seconds
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if request.DeviceName == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}

	// Default timeout to 5 seconds
	if request.Timeout <= 0 {
		request.Timeout = 5
	}

	// Load configuration to find the device
	cfgData, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to load configuration", err)
		return
	}

	// Find the device
	var device *config.DeviceData
	for _, d := range cfgData.Devices {
		if d.Name == request.DeviceName {
			device = &d
			break
		}
	}

	if device == nil {
		h.sendError(w, http.StatusNotFound, "Device not found", nil)
		return
	}

	// Test connectivity based on device type and connection method
	result := h.testDeviceConnection(device, request.Timeout)

	response := map[string]interface{}{
		"device_name": request.DeviceName,
		"device_type": device.Type,
		"success":     result.Success,
		"message":     result.Message,
		"duration_ms": result.DurationMs,
		"timestamp":   time.Now().Unix(),
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	h.sendJSON(w, response)
}

type ConnectivityTestResult struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func (h *Handlers) testDeviceConnection(device *config.DeviceData, timeoutSeconds int) ConnectivityTestResult {
	start := time.Now()
	timeout := time.Duration(timeoutSeconds) * time.Second

	// Test based on connection type
	if device.Hostname != "" && device.Port != "" {
		// Network connection (TCP)
		return h.testTCPConnection(device, timeout, start)
	} else if device.SerialDevice != "" {
		// Serial connection
		return h.testSerialConnection(device, timeout, start)
	} else {
		return ConnectivityTestResult{
			Success:    false,
			Message:    "Device has no valid connection configuration",
			Error:      "Neither hostname/port nor serial device configured",
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
}

func (h *Handlers) testTCPConnection(device *config.DeviceData, timeout time.Duration, start time.Time) ConnectivityTestResult {
	address := net.JoinHostPort(device.Hostname, device.Port)

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return ConnectivityTestResult{
			Success:    false,
			Message:    fmt.Sprintf("Failed to connect to %s", address),
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer conn.Close()

	return ConnectivityTestResult{
		Success:    true,
		Message:    fmt.Sprintf("Successfully connected to %s (%s)", address, device.Type),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func (h *Handlers) testSerialConnection(device *config.DeviceData, timeout time.Duration, start time.Time) ConnectivityTestResult {
	baud := device.Baud
	if baud <= 0 {
		baud = 9600 // Default baud rate
	}

	config := &serial.Config{
		Name:        device.SerialDevice,
		Baud:        baud,
		ReadTimeout: timeout,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		return ConnectivityTestResult{
			Success:    false,
			Message:    fmt.Sprintf("Failed to open serial port %s", device.SerialDevice),
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer port.Close()

	return ConnectivityTestResult{
		Success:    true,
		Message:    fmt.Sprintf("Successfully opened serial port %s at %d baud (%s)", device.SerialDevice, baud, device.Type),
		DurationMs: time.Since(start).Milliseconds(),
	}
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

// TestAPIConnectivity tests connectivity to external weather APIs
func (h *Handlers) TestAPIConnectivity(w http.ResponseWriter, r *http.Request) {
	var request struct {
		APIType string `json:"api_type"` // "pwsweather", "wunderground", "aerisweather"
		Timeout int    `json:"timeout,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if request.APIType == "" {
		h.sendError(w, http.StatusBadRequest, "API type is required", nil)
		return
	}

	if request.Timeout <= 0 {
		request.Timeout = 10 // Default 10 seconds for API calls
	}

	// Load configuration to find API settings
	cfgData, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to load configuration", err)
		return
	}

	// Find the API controller configuration
	var apiConfig interface{}
	var endpoint string

	for _, controller := range cfgData.Controllers {
		switch request.APIType {
		case "pwsweather":
			if controller.PWSWeather != nil {
				apiConfig = controller.PWSWeather
				if controller.PWSWeather.APIEndpoint != "" {
					endpoint = controller.PWSWeather.APIEndpoint
				} else {
					endpoint = "https://www.pwsweather.com/pwsweather/restapi" // Default
				}
			}
		case "wunderground":
			if controller.WeatherUnderground != nil {
				apiConfig = controller.WeatherUnderground
				if controller.WeatherUnderground.APIEndpoint != "" {
					endpoint = controller.WeatherUnderground.APIEndpoint
				} else {
					endpoint = "https://rtupdate.wunderground.com/weatherstation/updateweatherstation.php" // Default
				}
			}
		case "aerisweather":
			if controller.AerisWeather != nil {
				apiConfig = controller.AerisWeather
				if controller.AerisWeather.APIEndpoint != "" {
					endpoint = controller.AerisWeather.APIEndpoint
				} else {
					endpoint = "https://api.aerisapi.com" // Default
				}
			}
		}
	}

	if apiConfig == nil {
		h.sendError(w, http.StatusNotFound, fmt.Sprintf("%s API not configured", request.APIType), nil)
		return
	}

	// Test API connectivity
	result := h.testAPIEndpoint(endpoint, request.Timeout)

	response := map[string]interface{}{
		"api_type":    request.APIType,
		"endpoint":    endpoint,
		"success":     result.Success,
		"message":     result.Message,
		"duration_ms": result.DurationMs,
		"timestamp":   time.Now().Unix(),
	}

	if result.Error != "" {
		response["error"] = result.Error
	}

	h.sendJSON(w, response)
}

func (h *Handlers) testAPIEndpoint(endpoint string, timeoutSeconds int) ConnectivityTestResult {
	start := time.Now()
	timeout := time.Duration(timeoutSeconds) * time.Second

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return ConnectivityTestResult{
			Success:    false,
			Message:    fmt.Sprintf("Failed to connect to API endpoint %s", endpoint),
			Error:      err.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	statusClass := resp.StatusCode / 100
	if statusClass == 2 || statusClass == 3 || statusClass == 4 {
		// 2xx, 3xx, or 4xx are all considered "reachable" (server responded)
		return ConnectivityTestResult{
			Success:    true,
			Message:    fmt.Sprintf("API endpoint reachable (HTTP %d)", resp.StatusCode),
			DurationMs: time.Since(start).Milliseconds(),
		}
	} else {
		return ConnectivityTestResult{
			Success:    false,
			Message:    fmt.Sprintf("API endpoint returned HTTP %d", resp.StatusCode),
			Error:      fmt.Sprintf("Unexpected HTTP status: %d", resp.StatusCode),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
}
