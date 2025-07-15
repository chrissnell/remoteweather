package management

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/pkg/config"
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

	// Special handling for ambient-customized devices
	if device.Type == "ambient-customized" {
		return h.testAmbientCustomizedConnection(device, timeout, start)
	}

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

func (h *Handlers) testAmbientCustomizedConnection(device *config.DeviceData, _ time.Duration, start time.Time) ConnectivityTestResult {
	// For ambient-customized devices, we can't test connectivity in the traditional sense
	// since they run as HTTP servers waiting for incoming requests
	// Instead, we'll validate the configuration

	if device.Port == "" {
		return ConnectivityTestResult{
			Success:    false,
			Message:    "Port is required for Ambient Weather Customized Server",
			Error:      "Missing port configuration",
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	listenAddr := "0.0.0.0"
	if device.Hostname != "" {
		listenAddr = device.Hostname
	}

	path := device.Path
	if path == "" {
		path = "/"
	}

	return ConnectivityTestResult{
		Success:    true,
		Message:    fmt.Sprintf("Ambient Weather Customized Server configured to listen on %s:%s, path %s", listenAddr, device.Port, path),
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
	if storage.TimescaleDB != nil && storage.TimescaleDB.GetConnectionString() != "" {
		connected := false
		errorMessage := ""

		// Try to connect to TimescaleDB using internal database package
		db, err := database.CreateConnection(storage.TimescaleDB.GetConnectionString())
		if err != nil {
			errorMessage = err.Error()
		} else {
			sqlDB, err := db.DB()
			if err != nil {
				errorMessage = err.Error()
			} else {
				defer sqlDB.Close()
				if err := sqlDB.Ping(); err != nil {
					errorMessage = err.Error()
				} else {
					connected = true
				}
			}
		}

		testResults = append(testResults, map[string]interface{}{
			"database":        "timescaledb",
			"connected":       connected,
			"error":           errorMessage,
			"config_provided": true,
		})
	} else {
		testResults = append(testResults, map[string]interface{}{
			"database":        "timescaledb",
			"connected":       false,
			"error":           "No TimescaleDB configuration found",
			"config_provided": false,
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
					endpoint = "https://pwsupdate.pwsweather.com/api/v1/submitwx" // Default
				}
			}
		case "wunderground":
			if controller.WeatherUnderground != nil {
				apiConfig = controller.WeatherUnderground
				if controller.WeatherUnderground.APIEndpoint != "" {
					endpoint = controller.WeatherUnderground.APIEndpoint
				} else {
					endpoint = "https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php" // Default
				}
			}
		case "aerisweather":
			if controller.AerisWeather != nil {
				apiConfig = controller.AerisWeather
				if controller.AerisWeather.APIEndpoint != "" {
					endpoint = controller.AerisWeather.APIEndpoint
				} else {
					endpoint = "https://data.api.xweather.com" // Default
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

// GetCurrentWeatherReading gets current weather readings from a specific device
func (h *Handlers) GetCurrentWeatherReading(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceName      string `json:"device_name"`
		MaxStaleMinutes int    `json:"max_stale_minutes,omitempty"` // minutes to consider reading stale
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if request.DeviceName == "" {
		h.sendError(w, http.StatusBadRequest, "Device name is required", nil)
		return
	}

	// Default stale threshold to 15 minutes
	if request.MaxStaleMinutes <= 0 {
		request.MaxStaleMinutes = 15
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

	// Get current weather reading from database
	reading, err := h.getCurrentWeatherFromDatabase(request.DeviceName, request.MaxStaleMinutes)
	if err != nil {
		h.sendError(w, http.StatusServiceUnavailable, "Failed to get weather reading", err)
		return
	}

	response := map[string]interface{}{
		"device_name":         request.DeviceName,
		"device_type":         device.Type,
		"reading":             reading.Reading,
		"timestamp":           reading.Timestamp.Unix(),
		"reading_age_minutes": reading.AgeMinutes,
		"is_stale":            reading.IsStale,
		"success":             true,
	}

	h.sendJSON(w, response)
}

// WeatherReadingResult represents a weather reading from the database with metadata
type WeatherReadingResult struct {
	Reading    map[string]interface{} `json:"reading"`
	Timestamp  time.Time              `json:"timestamp"`
	AgeMinutes int                    `json:"age_minutes"`
	IsStale    bool                   `json:"is_stale"`
}

// getCurrentWeatherFromDatabase retrieves the most recent weather reading from the database
func (h *Handlers) getCurrentWeatherFromDatabase(deviceName string, maxStaleMinutes int) (*WeatherReadingResult, error) {
	// Get storage configuration to determine if TimescaleDB is available
	storage, err := h.controller.ConfigProvider.GetStorageConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	if storage.TimescaleDB == nil || storage.TimescaleDB.GetConnectionString() == "" {
		return nil, fmt.Errorf("TimescaleDB storage not configured - cannot retrieve weather readings")
	}

	// Connect to TimescaleDB using internal database package
	gormDB, err := database.CreateConnection(storage.TimescaleDB.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL database: %w", err)
	}
	defer sqlDB.Close()

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database connection test failed: %w", err)
	}

	// Query for the most recent reading from the specified station
	query := `
		SELECT time, stationname, stationtype, barometer, intemp, inhumidity, outtemp, 
		       windspeed, windspeed10, winddir, windchill, heatindex, outhumidity,
		       rainrate, rainincremental, solarwatts, potentialsolarwatts, uv, radiation,
		       stormrain, dayrain, monthrain, yearrain, snowdistance, snowdepth,
		       consbatteryvoltage, stationbatteryvoltage
		FROM weather 
		WHERE stationname = $1 
		ORDER BY time DESC 
		LIMIT 1
	`

	var timestamp time.Time
	var stationName, stationType string
	var barometer, inTemp, inHumidity, outTemp sql.NullFloat64
	var windSpeed, windSpeed10, windDir, windChill, heatIndex sql.NullFloat64
	var outHumidity, rainRate, rainIncremental sql.NullFloat64
	var solarWatts, potentialSolarWatts, uv, radiation sql.NullFloat64
	var stormRain, dayRain, monthRain, yearRain sql.NullFloat64
	var snowDistance, snowDepth sql.NullFloat64
	var consBatteryVoltage, stationBatteryVoltage sql.NullFloat64

	err = sqlDB.QueryRow(query, deviceName).Scan(
		&timestamp, &stationName, &stationType,
		&barometer, &inTemp, &inHumidity, &outTemp,
		&windSpeed, &windSpeed10, &windDir, &windChill, &heatIndex,
		&outHumidity, &rainRate, &rainIncremental,
		&solarWatts, &potentialSolarWatts, &uv, &radiation,
		&stormRain, &dayRain, &monthRain, &yearRain,
		&snowDistance, &snowDepth,
		&consBatteryVoltage, &stationBatteryVoltage,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no weather readings found for device %s", deviceName)
		}
		return nil, fmt.Errorf("failed to query weather readings: %w", err)
	}

	// Calculate age of the reading
	ageMinutes := int(time.Since(timestamp).Minutes())
	isStale := ageMinutes > maxStaleMinutes

	// Build the reading map with available data
	reading := map[string]interface{}{
		"station_name": stationName,
		"station_type": stationType,
	}

	// Add non-null values to the reading
	if barometer.Valid {
		reading["barometer_inhg"] = barometer.Float64
	}
	if inTemp.Valid {
		reading["inside_temperature_f"] = inTemp.Float64
	}
	if inHumidity.Valid {
		reading["inside_humidity_percent"] = inHumidity.Float64
	}
	if outTemp.Valid {
		reading["outside_temperature_f"] = outTemp.Float64
	}
	if windSpeed.Valid {
		reading["wind_speed_mph"] = windSpeed.Float64
	}
	if windSpeed10.Valid {
		reading["wind_speed_10min_avg_mph"] = windSpeed10.Float64
	}
	if windDir.Valid {
		reading["wind_direction_degrees"] = windDir.Float64
	}
	if windChill.Valid {
		reading["wind_chill_f"] = windChill.Float64
	}
	if heatIndex.Valid {
		reading["heat_index_f"] = heatIndex.Float64
	}
	if outHumidity.Valid {
		reading["outside_humidity_percent"] = outHumidity.Float64
	}
	if rainRate.Valid {
		reading["rain_rate_in_per_hr"] = rainRate.Float64
	}
	if rainIncremental.Valid {
		reading["rain_incremental_in"] = rainIncremental.Float64
	}
	if solarWatts.Valid {
		reading["solar_watts"] = solarWatts.Float64
	}
	if potentialSolarWatts.Valid {
		reading["potential_solar_watts"] = potentialSolarWatts.Float64
	}
	if uv.Valid {
		reading["uv_index"] = uv.Float64
	}
	if radiation.Valid {
		reading["solar_radiation_wm2"] = radiation.Float64
	}
	if stormRain.Valid {
		reading["storm_rain_in"] = stormRain.Float64
	}
	if dayRain.Valid {
		reading["day_rain_in"] = dayRain.Float64
	}
	if monthRain.Valid {
		reading["month_rain_in"] = monthRain.Float64
	}
	if yearRain.Valid {
		reading["year_rain_in"] = yearRain.Float64
	}
	if snowDistance.Valid {
		reading["snow_distance_in"] = snowDistance.Float64
	}
	if snowDepth.Valid {
		reading["snow_depth_in"] = snowDepth.Float64
	}
	if consBatteryVoltage.Valid {
		reading["console_battery_voltage"] = consBatteryVoltage.Float64
	}
	if stationBatteryVoltage.Valid {
		reading["station_battery_voltage"] = stationBatteryVoltage.Float64
	}

	return &WeatherReadingResult{
		Reading:    reading,
		Timestamp:  timestamp,
		AgeMinutes: ageMinutes,
		IsStale:    isStale,
	}, nil
}

// GetStorageHealthStatus returns the health status of all storage backends
func (h *Handlers) GetStorageHealthStatus(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	// Get all storage health statuses
	healthMap, err := h.controller.ConfigProvider.GetAllStorageHealth()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get storage health status", err)
		return
	}

	// Convert to response format with calculated staleness
	var healthStatuses []map[string]interface{}
	for storageType, health := range healthMap {
		status := map[string]interface{}{
			"storage_type": storageType,
			"health":       health,
		}

		if health != nil {
			// Calculate if health data is stale (older than 5 minutes)
			if !health.LastCheck.IsZero() {
				ageMinutes := int(time.Since(health.LastCheck).Minutes())
				status["age_minutes"] = ageMinutes
				status["is_stale"] = ageMinutes > 5
			} else {
				status["age_minutes"] = nil
				status["is_stale"] = true
			}

			// Determine overall health status
			if health.Status == "" {
				status["overall_status"] = "unknown"
			} else if health.Status == "healthy" && status["is_stale"].(bool) {
				status["overall_status"] = "stale"
			} else {
				status["overall_status"] = health.Status
			}
		} else {
			status["overall_status"] = "unknown"
			status["age_minutes"] = nil
			status["is_stale"] = true
		}

		healthStatuses = append(healthStatuses, status)
	}

	response := map[string]interface{}{
		"storage_health": healthStatuses,
		"timestamp":      time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// GetSingleStorageHealth returns the health status of a specific storage backend
func (h *Handlers) GetSingleStorageHealth(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	// Get storage type from URL path
	storageType := strings.TrimPrefix(r.URL.Path, "/api/health/storage/")
	if storageType == "" {
		h.sendError(w, http.StatusBadRequest, "Storage type is required in URL path", nil)
		return
	}

	// Get health status for specific storage type
	health, err := h.controller.ConfigProvider.GetStorageHealth(storageType)
	if err != nil {
		h.sendError(w, http.StatusNotFound, fmt.Sprintf("Failed to get health for storage type '%s'", storageType), err)
		return
	}

	status := map[string]interface{}{
		"storage_type": storageType,
		"health":       health,
	}

	if health != nil {
		// Calculate if health data is stale (older than 5 minutes)
		if !health.LastCheck.IsZero() {
			ageMinutes := int(time.Since(health.LastCheck).Minutes())
			status["age_minutes"] = ageMinutes
			status["is_stale"] = ageMinutes > 5
		} else {
			status["age_minutes"] = nil
			status["is_stale"] = true
		}

		// Determine overall health status
		if health.Status == "" {
			status["overall_status"] = "unknown"
		} else if health.Status == "healthy" && status["is_stale"].(bool) {
			status["overall_status"] = "stale"
		} else {
			status["overall_status"] = health.Status
		}
	} else {
		status["overall_status"] = "unknown"
		status["age_minutes"] = nil
		status["is_stale"] = true
	}

	response := map[string]interface{}{
		"storage_health": status,
		"timestamp":      time.Now().Unix(),
	}

	h.sendJSON(w, response)
}

// TestStorageConnectivity tests connectivity to storage backends (compatibility endpoint for frontend)
func (h *Handlers) TestStorageConnectivity(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	// Get all storage health statuses
	healthMap, err := h.controller.ConfigProvider.GetAllStorageHealth()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to get storage health status", err)
		return
	}

	// Convert to frontend-compatible format
	var storageResults []map[string]interface{}
	for storageType, health := range healthMap {
		connected := false
		if health != nil && health.Status == "healthy" {
			// Consider healthy and not stale as connected
			ageMinutes := int(time.Since(health.LastCheck).Minutes())
			connected = ageMinutes <= 5
		}

		storageResults = append(storageResults, map[string]interface{}{
			"name":      storageType,
			"connected": connected,
			"status":    health,
		})
	}

	response := map[string]interface{}{
		"storage":   storageResults,
		"timestamp": time.Now().Unix(),
	}

	h.sendJSON(w, response)
}
