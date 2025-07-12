package restserver

import (
	"encoding/json"
	htmltemplate "html/template"
	"net/http"
	"regexp"
	"text/template"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Handlers contains all HTTP handlers for the REST server
type Handlers struct {
	controller *Controller
}

// NewHandlers creates a new handlers instance
func NewHandlers(ctrl *Controller) *Handlers {
	return &Handlers{
		controller: ctrl,
	}
}

// getWebsiteFromContext extracts the website from request context
func (h *Handlers) getWebsiteFromContext(req *http.Request) *config.WeatherWebsiteData {
	if website, ok := req.Context().Value(websiteContextKey).(*config.WeatherWebsiteData); ok {
		return website
	}
	// Fallback to default website if context is missing
	return h.controller.DefaultWebsite
}

// getPrimaryDeviceForWebsite returns the primary device associated with a website
// Prefers enabled devices, but falls back to disabled devices if no enabled devices are available
func (h *Handlers) getPrimaryDeviceForWebsite(website *config.WeatherWebsiteData) string {
	devices := h.controller.DevicesByWebsite[website.ID]
	if len(devices) == 0 {
		return ""
	}

	// First, try to find an enabled device
	for _, device := range devices {
		if device.Enabled {
			return device.Name
		}
	}

	// If no enabled devices, fall back to first device (for historical data access)
	return devices[0].Name
}

// getSnowBaseDistance returns the snow base distance from the snow device configuration for a website
func (h *Handlers) getSnowBaseDistance(website *config.WeatherWebsiteData) float32 {
	if website.SnowDeviceName != "" {
		for _, device := range h.controller.Devices {
			if device.Name == website.SnowDeviceName {
				return float32(device.BaseSnowDistance)
			}
		}
	}
	return 0.0
}

// validateStationExists checks if a station name exists in the configuration
func (h *Handlers) validateStationExists(stationName string) bool {
	if stationName == "" {
		return false
	}

	// Use O(1) map lookup instead of O(n) linear search
	return h.controller.DeviceNames[stationName]
}

// GetWeatherSpan handles requests for weather data over a time span
func (h *Handlers) GetWeatherSpan(w http.ResponseWriter, req *http.Request) {
	if h.controller.DBEnabled {
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Info)
		} else {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Warn)
		}

		var dbFetchedReadings []types.BucketReading

		stationName := req.URL.Query().Get("station")

		// Require station parameter
		if stationName == "" {
			http.Error(w, "station parameter is required", http.StatusBadRequest)
			return
		}

		// Validate station name
		if !h.validateStationExists(stationName) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}

		vars := mux.Vars(req)
		span, err := time.ParseDuration(vars["span"])
		if err != nil {
			log.Errorf("invalid request: unable to parse duration: %v", vars["span"])
			http.Error(w, "error: invalid span duration", http.StatusBadRequest)
			return
		}

		spanStart := time.Now().Add(-span)
		// Get website from context and snow base distance
		website := h.getWebsiteFromContext(req)
		baseDistance := h.getSnowBaseDistance(website)

		switch {
		case span < 1*Day:
			h.controller.DB.Table("weather_1m").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > ?", spanStart).
				Where("stationname = ?", stationName).
				Order("bucket").
				Find(&dbFetchedReadings)
		case span >= 1*Day && span < 7*Day:
			h.controller.DB.Table("weather_5m").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > ?", spanStart).
				Where("stationname = ?", stationName).
				Order("bucket").
				Find(&dbFetchedReadings)
		case span >= 7*Day && span < 2*Month:
			h.controller.DB.Table("weather_1h").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > ?", spanStart).
				Where("stationname = ?", stationName).
				Order("bucket").
				Find(&dbFetchedReadings)
		default:
			h.controller.DB.Table("weather_1h").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > ?", spanStart).
				Where("stationname = ?", stationName).
				Order("bucket").
				Find(&dbFetchedReadings)
		}

		spanReadings := h.transformSpanReadings(&dbFetchedReadings)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		err = json.NewEncoder(w).Encode(spanReadings)
		if err != nil {
			log.Error("error encoding weather span readings to JSON:", err)
			return
		}
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// GetWeatherLatest handles requests for the latest weather data
func (h *Handlers) GetWeatherLatest(w http.ResponseWriter, req *http.Request) {
	if h.controller.DBEnabled {
		var dbFetchedReadings []types.BucketReading

		stationName := req.URL.Query().Get("station")

		// Validate station name if provided
		if stationName != "" && !h.validateStationExists(stationName) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}

		// Get website from context and find primary device
		website := h.getWebsiteFromContext(req)
		primaryDevice := h.getPrimaryDeviceForWebsite(website)

		if stationName != "" {
			h.controller.DB.Table("weather").Limit(1).Where("stationname = ?", stationName).Order("time DESC").Find(&dbFetchedReadings)
		} else {
			// Client did not supply a station name, so pull from the configured primary device
			h.controller.DB.Table("weather").Limit(1).Where("stationname = ?", primaryDevice).Order("time DESC").Find(&dbFetchedReadings)
		}

		latestReading := h.transformLatestReadings(&dbFetchedReadings)

		// Add total rainfall for the day
		type Rainfall struct {
			TotalRain float32
		}

		var totalRainfall Rainfall
		// Fetch the rainfall since midnight
		h.controller.DB.Table("today_rainfall").First(&totalRainfall)

		// Override DayRain from our weather table with the latest data from our view
		if len(dbFetchedReadings) > 0 {
			latestReading.RainfallDay = float32ToJSONNumber(totalRainfall.TotalRain)
		}

		// Calculate rainfall totals for different time periods
		if len(dbFetchedReadings) > 0 {
			stationName := dbFetchedReadings[0].StationName

			// 24-hour rainfall total
			var rainfall24h Rainfall
			h.controller.DB.Raw(`
				SELECT COALESCE(SUM(period_rain), 0) as total_rain
				FROM weather_5m 
				WHERE stationname = ? AND bucket >= NOW() - INTERVAL '24 hours'
			`, stationName).Scan(&rainfall24h)
			latestReading.Rainfall24h = float32ToJSONNumber(rainfall24h.TotalRain)

			// 48-hour rainfall total
			var rainfall48h Rainfall
			h.controller.DB.Raw(`
				SELECT COALESCE(SUM(period_rain), 0) as total_rain
				FROM weather_5m 
				WHERE stationname = ? AND bucket >= NOW() - INTERVAL '48 hours'
			`, stationName).Scan(&rainfall48h)
			latestReading.Rainfall48h = float32ToJSONNumber(rainfall48h.TotalRain)

			// 72-hour rainfall total
			var rainfall72h Rainfall
			h.controller.DB.Raw(`
				SELECT COALESCE(SUM(period_rain), 0) as total_rain
				FROM weather_5m 
				WHERE stationname = ? AND bucket >= NOW() - INTERVAL '72 hours'
			`, stationName).Scan(&rainfall72h)
			latestReading.Rainfall72h = float32ToJSONNumber(rainfall72h.TotalRain)

			// Storm rainfall total using existing function
			type StormRainResult struct {
				StormStart    *time.Time `gorm:"column:storm_start"`
				StormEnd      *time.Time `gorm:"column:storm_end"`
				TotalRainfall float32    `gorm:"column:total_rainfall"`
			}
			var stormResult StormRainResult
			err := h.controller.DB.Raw("SELECT * FROM calculate_storm_rainfall(?) LIMIT 1", stationName).Scan(&stormResult).Error
			if err != nil {
				log.Errorf("error getting storm rainfall from DB: %v", err)
			} else {
				latestReading.RainfallStorm = float32ToJSONNumber(stormResult.TotalRainfall)
			}
		}

		// Calculate wind gust from last 10 minutes
		if len(dbFetchedReadings) > 0 {
			type WindGustResult struct {
				WindGust float32
			}
			var windGustResult WindGustResult
			query := "SELECT calculate_wind_gust(?) AS wind_gust"
			err := h.controller.DB.Raw(query, dbFetchedReadings[0].StationName).Scan(&windGustResult).Error
			if err != nil {
				log.Errorf("error getting wind gust from DB: %v", err)
			} else {
				latestReading.WindGust = float32ToJSONNumber(windGustResult.WindGust)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		err := json.NewEncoder(w).Encode(latestReading)
		if err != nil {
			log.Error("error encoding latest weather readings to JSON:", err)
			return
		}
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// GetSnowLatest handles requests for the latest snow data
func (h *Handlers) GetSnowLatest(w http.ResponseWriter, req *http.Request) {
	// Get website from context and check if snow is enabled
	website := h.getWebsiteFromContext(req)
	if !website.SnowEnabled {
		http.Error(w, "snow data not enabled for this website", http.StatusNotFound)
		return
	}

	if h.controller.DBEnabled {
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Info)
		} else {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Warn)
		}

		var dbFetchedReadings []types.BucketReading

		stationName := req.URL.Query().Get("station")

		// Validate station name if provided
		if stationName != "" && !h.validateStationExists(stationName) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}

		if stationName != "" {
			h.controller.DB.Table("weather").Limit(1).Where("stationname = ?", stationName).Order("time DESC").Find(&dbFetchedReadings)
		} else {
			// Client did not supply a station name, so pull from the configured snow device
			h.controller.DB.Table("weather").Limit(1).Where("stationname = ?", website.SnowDeviceName).Order("time DESC").Find(&dbFetchedReadings)
		}

		log.Debugf("returned rows: %v", len(dbFetchedReadings))

		if len(dbFetchedReadings) > 0 {
			log.Debugf("latest snow reading: %v", mmToInches(dbFetchedReadings[0].SnowDistance))
		}

		var result SnowDeltaResult

		// Get snow base distance from the snow device
		snowBaseDistance := h.getSnowBaseDistance(website)

		// Get the snowfall since midnight
		query := "SELECT get_new_snow_midnight(?, ?) AS snowfall"
		err := h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting snow-since-midnight snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Snow since midnight: %.2f mm\n", result.Snowfall)
		snowSinceMidnight := mmToInches(result.Snowfall)

		// Get the snowfall in the last 24 hours
		query = "SELECT get_new_snow_24h(?, ?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 24-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Snow in last 24h: %.2f mm\n", result.Snowfall)
		snowLast24 := mmToInches(result.Snowfall)

		// Get the snowfall in the last 72 hours
		query = "SELECT get_new_snow_72h(?, ?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 72-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Snow in last 72h: %.2f mm\n", result.Snowfall)
		snowLast72 := mmToInches(result.Snowfall)

		// Get the season total snowfall
		query = "SELECT calculate_total_season_snowfall(?, ?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting season total snowfall from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Season total snowfall: %.2f mm\n", result.Snowfall)
		snowSeason := mmToInches(result.Snowfall)

		// Get the storm total snowfall
		type StormResult struct {
			TotalSnowfall float32 `gorm:"column:total_snowfall"`
		}
		var stormResult StormResult
		query = "SELECT * FROM calculate_storm_snowfall(?) LIMIT 1"
		err = h.controller.DB.Raw(query, website.SnowDeviceName).Scan(&stormResult).Error
		if err != nil {
			log.Errorf("error getting storm total snowfall from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Storm total snowfall: %.2f mm\n", stormResult.TotalSnowfall)
		snowStorm := mmToInches(stormResult.TotalSnowfall)

		// Get the current snowfall rate
		query = "SELECT calculate_current_snowfall_rate(?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting current snowfall rate from DB: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}
		log.Debugf("Current snowfall rate: %.2f mm/hr\n", result.Snowfall)
		snowfallRate := mmToInches(result.Snowfall)

		// Check if we have any readings from the snow gauge
		var snowDepth float32 = 0.0
		if len(dbFetchedReadings) > 0 {
			snowDepth = mmToInches(snowBaseDistance - dbFetchedReadings[0].SnowDistance)
		} else {
			log.Warnf("No readings available from snow device '%s' - returning zero values", website.SnowDeviceName)
		}

		snowReading := SnowReading{
			StationName:  website.SnowDeviceName,
			SnowDepth:    snowDepth,
			SnowToday:    float32(snowSinceMidnight),
			SnowLast24:   float32(snowLast24),
			SnowLast72:   float32(snowLast72),
			SnowSeason:   float32(snowSeason),
			SnowStorm:    float32(snowStorm),
			SnowfallRate: float32(snowfallRate),
		}

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(&snowReading)
		if err != nil {
			log.Errorf("error marshalling snowReading: %v", err)
			http.Error(w, "error fetching readings from DB", http.StatusInternalServerError)
			return
		}

		w.Write(jsonResponse)
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// GetForecast handles requests for forecast data
func (h *Handlers) GetForecast(w http.ResponseWriter, req *http.Request) {
	if h.controller.DBEnabled {
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Info)
		} else {
			h.controller.DB.Logger = h.controller.DB.Logger.LogMode(logger.Warn)
		}

		vars := mux.Vars(req)
		span := vars["span"]
		if span == "" {
			log.Errorf("invalid request: missing span duration")
			http.Error(w, "error: missing span duration", http.StatusBadRequest)
			return
		}

		// 'span' must be between 1 and 4 digits and nothing else
		re := regexp.MustCompile(`^\d{1,4}$`)
		if !re.MatchString(span) {
			log.Errorf("span %v is invalid", span)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		location := req.URL.Query().Get("location")

		record := AerisWeatherForecastRecord{}

		var result *gorm.DB
		if location != "" {
			result = h.controller.DB.Where("forecast_span_hours = ? AND location = ?", span, location).First(&record)
		} else {
			result = h.controller.DB.Where("forecast_span_hours = ?", span).First(&record)
		}
		if result.RowsAffected == 0 {
			log.Errorf("no forecast records found for span %v", span)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"lastUpdated\": \"" + record.UpdatedAt.String() + "\", \"data\": "))
		w.Write(record.Data.Bytes)
		w.Write([]byte("}"))
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// ServePortal serves the portal template for weather management portal websites
func (h *Handlers) ServePortal(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)

	// Only serve portal if this website is configured as a portal
	if !website.IsPortal {
		http.NotFound(w, req)
		return
	}

	view := htmltemplate.Must(htmltemplate.New("portal.html.tmpl").ParseFS(*h.controller.FS, "portal.html.tmpl"))

	// Portal doesn't need specific station data - it loads all stations via API
	templateData := struct {
		PageTitle string
	}{
		PageTitle: website.PageTitle,
	}

	w.Header().Set("Content-Type", "text/html")
	err := view.Execute(w, templateData)
	if err != nil {
		log.Error("error executing portal template:", err)
		return
	}
}

// ServeWeatherWebsiteTemplate serves the weather HTML template
func (h *Handlers) ServeWeatherWebsiteTemplate(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)

	// If this is a portal website, serve the portal instead
	if website.IsPortal {
		h.ServePortal(w, req)
		return
	}

	primaryDevice := h.getPrimaryDeviceForWebsite(website)

	view := htmltemplate.Must(htmltemplate.New("weather-station.html.tmpl").ParseFS(*h.controller.FS, "weather-station.html.tmpl"))

	// Create a template data structure with AboutStationHTML as safe HTML
	templateData := struct {
		StationName      string
		PullFromDevice   string
		SnowEnabled      bool
		SnowDevice       string
		SnowBaseDistance float32
		PageTitle        string
		AboutStationHTML htmltemplate.HTML
	}{
		StationName:      website.Name,
		PullFromDevice:   primaryDevice,
		SnowEnabled:      website.SnowEnabled,
		SnowDevice:       website.SnowDeviceName,
		SnowBaseDistance: h.getSnowBaseDistance(website),
		PageTitle:        website.PageTitle,
		AboutStationHTML: htmltemplate.HTML(website.AboutStationHTML),
	}

	w.Header().Set("Content-Type", "text/html")
	err := view.Execute(w, templateData)
	if err != nil {
		log.Error("error executing weather template:", err)
		return
	}
}

// ServeWeatherAppJS serves the weather app JavaScript template
func (h *Handlers) ServeWeatherAppJS(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)
	primaryDevice := h.getPrimaryDeviceForWebsite(website)

	view := template.Must(template.New("weather-app.js.tmpl").ParseFS(*h.controller.FS, "js/weather-app.js.tmpl"))

	// Create JS template data structure
	jsTemplateData := struct {
		StationName      string
		PullFromDevice   string
		SnowEnabled      bool
		SnowDevice       string
		SnowBaseDistance float32
		PageTitle        string
		AboutStationHTML string
	}{
		StationName:      website.Name,
		PullFromDevice:   primaryDevice,
		SnowEnabled:      website.SnowEnabled,
		SnowDevice:       website.SnowDeviceName,
		SnowBaseDistance: h.getSnowBaseDistance(website),
		PageTitle:        website.PageTitle,
		AboutStationHTML: website.AboutStationHTML,
	}

	w.Header().Set("Content-Type", "text/javascript")
	err := view.Execute(w, jsTemplateData)
	if err != nil {
		log.Error("error executing weather app JavaScript template:", err)
		return
	}
}

// GetStations returns all weather stations with their location data
func (h *Handlers) GetStations(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)

	// Only allow station API access for portal websites
	if !website.IsPortal {
		http.Error(w, "stations API not available for this website", http.StatusForbidden)
		return
	}

	// Load current devices from config provider to get latest configuration
	currentDevices, err := h.controller.configProvider.GetDevices()
	if err != nil {
		log.Error("error loading current devices from config:", err)
		http.Error(w, "error loading station data", http.StatusInternalServerError)
		return
	}

	// Load weather websites to find associations with devices
	websites, err := h.controller.configProvider.GetWeatherWebsites()
	if err != nil {
		log.Error("error loading weather websites from config:", err)
		http.Error(w, "error loading website data", http.StatusInternalServerError)
		return
	}

	// Create a map of device ID to weather website for quick lookup
	deviceToWebsite := make(map[int]*config.WeatherWebsiteData)
	for i := range websites {
		website := &websites[i]
		// Only include regular websites (not portals) that have a device association
		if !website.IsPortal && website.DeviceID != nil {
			deviceToWebsite[*website.DeviceID] = website
		}
	}

	// Get all devices with location data
	stations := make([]StationData, 0)

	for _, device := range currentDevices {
		// Only include devices that have location data
		if device.Latitude != 0 && device.Longitude != 0 {
			station := StationData{
				Name:      device.Name,
				Type:      device.Type,
				Latitude:  device.Latitude,
				Longitude: device.Longitude,
				Enabled:   device.Enabled,
			}

			// Check if this device has an associated weather website
			if websiteData, exists := deviceToWebsite[device.ID]; exists && websiteData.Hostname != "" {
				// Determine if website has TLS configured
				hasTLS := (websiteData.TLSCertPath != "" && websiteData.TLSKeyPath != "") ||
					(h.controller.restConfig.TLSCertPath != "" && h.controller.restConfig.TLSKeyPath != "")

				// Determine protocol and port
				var protocol string
				var port int

				if hasTLS {
					protocol = "https"
					if h.controller.restConfig.HTTPSPort != nil {
						port = *h.controller.restConfig.HTTPSPort
					} else {
						port = 443 // Standard HTTPS port
					}
				} else {
					protocol = "http"
					port = h.controller.restConfig.HTTPPort
					if port == 0 {
						port = 80 // Standard HTTP port
					}
				}

				station.Website = &StationWebsiteData{
					Name:      websiteData.Name,
					Hostname:  websiteData.Hostname,
					PageTitle: websiteData.PageTitle,
					Protocol:  protocol,
					Port:      port,
				}
			}

			stations = append(stations, station)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	err = json.NewEncoder(w).Encode(stations)
	if err != nil {
		log.Error("error encoding stations to JSON:", err)
		http.Error(w, "error encoding stations", http.StatusInternalServerError)
		return
	}
}
