package management

import (
	"net/http"
	"time"
)

// GetStatus returns the status of the management API
func (h *Handlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"message":   "Management API is running",
	}

	h.sendJSON(w, status)
}

// GetConfig returns the current configuration
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	if h.controller.ConfigProvider == nil {
		h.sendError(w, http.StatusServiceUnavailable, "No config provider available", nil)
		return
	}

	configData, err := h.controller.ConfigProvider.LoadConfig()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to load configuration", err)
		return
	}

	response := map[string]interface{}{
		"config":           configData,
		"read_only":        h.controller.ConfigProvider.IsReadOnly(),
		"timestamp":        time.Now().Unix(),
		"device_count":     len(configData.Devices),
		"controller_count": len(configData.Controllers),
	}

	h.sendJSON(w, response)
}

// ServeIndex serves the main management interface
func (h *Handlers) ServeIndex(w http.ResponseWriter, r *http.Request) {
	// For now, serve a simple HTML page
	html := `<!DOCTYPE html>
<html>
<head>
    <title>RemoteWeather Management</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 40px; 
            background-color: #f5f5f5; 
        }
        .container { 
            max-width: 800px; 
            margin: 0 auto; 
            background: white; 
            padding: 30px; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1); 
        }
        h1 { 
            color: #333; 
            text-align: center; 
        }
        .api-info { 
            background: #e8f4fd; 
            padding: 20px; 
            border-radius: 5px; 
            margin: 20px 0; 
        }
        .endpoint { 
            font-family: monospace; 
            background: #f0f0f0; 
            padding: 5px 10px; 
            border-radius: 3px; 
            margin: 5px 0; 
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>RemoteWeather Management API</h1>
        <p>Welcome to the RemoteWeather Management Interface.</p>
        
        <div class="api-info">
            <h3>System Discovery:</h3>
            <div class="endpoint">GET /api/status - API status</div>
            <div class="endpoint">GET /api/config - Current configuration</div>
            <div class="endpoint">GET /api/system/serial-ports - Available serial ports</div>
            <div class="endpoint">GET /api/system/info - System information</div>
        </div>
        
        <div class="api-info">
            <h3>Configuration Management:</h3>
            <div class="endpoint">GET /api/config/weather-stations - List weather stations</div>
            <div class="endpoint">POST /api/config/weather-stations - Create weather station</div>
            <div class="endpoint">PUT /api/config/weather-stations/{id} - Update weather station</div>
            <div class="endpoint">DELETE /api/config/weather-stations/{id} - Delete weather station</div>
            <div class="endpoint">GET /api/config/storage - List storage configs</div>
            <div class="endpoint">POST /api/config/storage - Create storage config</div>
            <div class="endpoint">PUT /api/config/storage/{id} - Update storage config</div>
            <div class="endpoint">DELETE /api/config/storage/{id} - Delete storage config</div>
        </div>
        
        <div class="api-info">
            <h3>Connectivity Testing:</h3>
            <div class="endpoint">POST /api/test/device - Test device connectivity</div>
            <div class="endpoint">GET /api/test/database - Test database connectivity</div>
            <div class="endpoint">GET /api/test/serial-port?port=/dev/ttyUSB0 - Test serial port connectivity</div>
            <div class="endpoint">POST /api/test/api - Test external API connectivity</div>
        </div>
        
        <p><strong>Note:</strong> API endpoints require authentication via Bearer token in the Authorization header.</p>
        
        <p>Management interface is currently under development. More features coming soon!</p>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
