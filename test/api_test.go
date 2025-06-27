package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// Test configuration
const (
	baseURL   = "http://127.0.0.1:8081"
	authToken = "test-token-123"
)

// Test data structures
type WeatherStation struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Hostname string `json:"hostname,omitempty"`
	Port     string `json:"port,omitempty"`
	Baud     int    `json:"baud,omitempty"`
	Solar    struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Altitude  float64 `json:"altitude"`
	} `json:"solar"`
}

// Helper function to make authenticated HTTP requests
func makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+authToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// Helper function to parse JSON response
func parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// Test API status endpoint
func TestAPIStatus(t *testing.T) {
	resp, err := makeRequest("GET", "/api/status", nil)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := parseResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", result["status"])
	}
}

// Test weather stations CRUD operations (WORKING)
func TestWeatherStationsCRUD(t *testing.T) {
	// Test data
	testStation := WeatherStation{
		Name:     "test-davis-station",
		Type:     "davis",
		Hostname: "192.168.1.200",
		Port:     "22222",
		Baud:     9600,
	}
	testStation.Solar.Latitude = 40.7128
	testStation.Solar.Longitude = -74.0060
	testStation.Solar.Altitude = 10.0

	updatedStation := testStation
	updatedStation.Hostname = "192.168.1.201"
	updatedStation.Port = "22223"
	updatedStation.Baud = 19200

	// CREATE: Add new weather station
	t.Run("Create Weather Station", func(t *testing.T) {
		resp, err := makeRequest("POST", "/api/config/weather-stations", testStation)
		if err != nil {
			t.Fatalf("Failed to create weather station: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d. Body: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["message"] != "Weather station created successfully" {
			t.Errorf("Unexpected response message: %v", result["message"])
		}

		// Validate creation by immediately querying the device
		endpoint := fmt.Sprintf("/api/config/weather-stations/%s", testStation.Name)
		getResp, err := makeRequest("GET", endpoint, nil)
		if err != nil {
			t.Fatalf("Failed to verify created weather station: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("Created weather station not found: status %d", getResp.StatusCode)
		}

		var createdDevice WeatherStation
		if err := parseResponse(getResp, &createdDevice); err != nil {
			t.Fatalf("Failed to parse created device: %v", err)
		}

		// Verify all fields were saved correctly
		if createdDevice.Name != testStation.Name {
			t.Errorf("Name mismatch: expected %s, got %s", testStation.Name, createdDevice.Name)
		}
		if createdDevice.Type != testStation.Type {
			t.Errorf("Type mismatch: expected %s, got %s", testStation.Type, createdDevice.Type)
		}
		if createdDevice.Hostname != testStation.Hostname {
			t.Errorf("Hostname mismatch: expected %s, got %s", testStation.Hostname, createdDevice.Hostname)
		}
		if createdDevice.Port != testStation.Port {
			t.Errorf("Port mismatch: expected %s, got %s", testStation.Port, createdDevice.Port)
		}
		if createdDevice.Baud != testStation.Baud {
			t.Errorf("Baud mismatch: expected %d, got %d", testStation.Baud, createdDevice.Baud)
		}
	})

	// READ: Get all weather stations
	t.Run("Read Weather Stations", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/config/weather-stations", nil)
		if err != nil {
			t.Fatalf("Failed to get weather stations: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		devices, ok := result["devices"].([]interface{})
		if !ok {
			t.Fatalf("Expected devices array, got %T", result["devices"])
		}

		found := false
		for _, device := range devices {
			deviceMap := device.(map[string]interface{})
			if deviceMap["name"] == testStation.Name {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Test station not found in devices list")
		}
	})

	// UPDATE: Modify weather station
	t.Run("Update Weather Station", func(t *testing.T) {
		endpoint := fmt.Sprintf("/api/config/weather-stations/%s", testStation.Name)
		resp, err := makeRequest("PUT", endpoint, updatedStation)
		if err != nil {
			t.Fatalf("Failed to update weather station: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["message"] != "Weather station updated successfully" {
			t.Errorf("Unexpected response message: %v", result["message"])
		}
	})

	// Verify update took effect by querying the specific device
	t.Run("Verify Update", func(t *testing.T) {
		endpoint := fmt.Sprintf("/api/config/weather-stations/%s", testStation.Name)
		resp, err := makeRequest("GET", endpoint, nil)
		if err != nil {
			t.Fatalf("Failed to get updated weather station: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var device WeatherStation
		if err := parseResponse(resp, &device); err != nil {
			t.Fatalf("Failed to parse device response: %v", err)
		}

		// Verify all updated fields
		if device.Hostname != updatedStation.Hostname {
			t.Errorf("Update failed: expected hostname %s, got %s", updatedStation.Hostname, device.Hostname)
		}
		if device.Port != updatedStation.Port {
			t.Errorf("Update failed: expected port %s, got %s", updatedStation.Port, device.Port)
		}
		if device.Baud != updatedStation.Baud {
			t.Errorf("Update failed: expected baud %d, got %d", updatedStation.Baud, device.Baud)
		}
		if device.Type != updatedStation.Type {
			t.Errorf("Update failed: expected type %s, got %s", updatedStation.Type, device.Type)
		}
	})

	// DELETE: Remove weather station
	t.Run("Delete Weather Station", func(t *testing.T) {
		endpoint := fmt.Sprintf("/api/config/weather-stations/%s", testStation.Name)
		resp, err := makeRequest("DELETE", endpoint, nil)
		if err != nil {
			t.Fatalf("Failed to delete weather station: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["message"] != "Weather station deleted successfully" {
			t.Errorf("Unexpected response message: %v", result["message"])
		}
	})

	// Verify deletion
	t.Run("Verify Deletion", func(t *testing.T) {
		endpoint := fmt.Sprintf("/api/config/weather-stations/%s", testStation.Name)
		resp, err := makeRequest("GET", endpoint, nil)
		if err != nil {
			t.Fatalf("Failed to get weather station: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 after deletion, got %d", resp.StatusCode)
		}
	})
}

// Test validation endpoints
func TestValidation(t *testing.T) {
	t.Run("Config Validation", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/config/validate", nil)
		if err != nil {
			t.Fatalf("Failed to validate config: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["valid"] != true {
			t.Errorf("Expected config to be valid, got %v", result["valid"])
		}
	})
}

// Test connectivity testing endpoints
func TestConnectivityTesting(t *testing.T) {
	t.Run("Device Connectivity Test", func(t *testing.T) {
		testRequest := map[string]interface{}{
			"device_name": "campbell-emulator",
			"timeout":     5,
		}

		resp, err := makeRequest("POST", "/api/test/device", testRequest)
		if err != nil {
			t.Fatalf("Failed to test device connectivity: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := parseResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if result["success"] != true {
			t.Errorf("Expected device test to succeed, got %v", result["success"])
		}
	})

	t.Run("Database Connectivity Test", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/test/database", nil)
		if err != nil {
			t.Fatalf("Failed to test database connectivity: %v", err)
		}
		defer resp.Body.Close()

		// Database test might fail if no database is configured, which is OK
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("Expected status 200 or 503, got %d", resp.StatusCode)
		}
	})
}

// Test error handling
func TestErrorHandling(t *testing.T) {
	t.Run("Invalid Authentication", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/api/status", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Authorization", "Bearer invalid-token")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Non-existent Device", func(t *testing.T) {
		resp, err := makeRequest("GET", "/api/config/weather-stations/non-existent", nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req, err := http.NewRequest("POST", baseURL+"/api/config/weather-stations", bytes.NewBufferString("invalid json"))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}
