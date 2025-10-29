package restserver

import (
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"regexp"
	"strconv"
	"text/template"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/responseformat"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Handlers contains all HTTP handlers for the REST server
type Handlers struct {
	controller *Controller
	formatter  *responseformat.Formatter
}

// NewHandlers creates a new handlers instance
func NewHandlers(ctrl *Controller) *Handlers {
	return &Handlers{
		controller: ctrl,
		formatter:  responseformat.NewFormatter(),
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

// getPrimaryDeviceForWebsite returns the primary device name associated with a website
func (h *Handlers) getPrimaryDeviceForWebsite(website *config.WeatherWebsiteData) string {
	device := h.getPrimaryDeviceConfigForWebsite(website)
	if device != nil {
		return device.Name
	}
	return ""
}

// getPrimaryDeviceConfigForWebsite returns the primary device configuration for a website
// This returns the first (and typically only) device for the website since websites
// are associated with a single device via device_id
func (h *Handlers) getPrimaryDeviceConfigForWebsite(website *config.WeatherWebsiteData) *config.DeviceData {
	if website == nil {
		return nil
	}
	devices := h.controller.DevicesByWebsite[website.ID]
	if len(devices) > 0 {
		return &devices[0]
	}
	return nil
}


// getAirQualityDeviceID returns the device ID for the air quality device
func (h *Handlers) getAirQualityDeviceID(website *config.WeatherWebsiteData) int {
	if website == nil || website.AirQualityDeviceName == "" {
		return 0
	}
	
	// Look up the device by name
	cfg, err := h.controller.configProvider.LoadConfig()
	if err != nil {
		return 0
	}
	
	for _, device := range cfg.Devices {
		if device.Name == website.AirQualityDeviceName {
			return device.ID
		}
	}
	
	return 0
}

// getSnowBaseDistance returns the cached snow base distance for a website
func (h *Handlers) getSnowBaseDistance(website *config.WeatherWebsiteData) float32 {
	if website == nil {
		return 0.0
	}
	// Return the pre-computed value from the cache
	return h.controller.SnowBaseDistanceCache[website.ID]
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
	// Check if any website is configured
	website := h.getWebsiteFromContext(req)
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "Weather data is not available until at least one weather website is configured",
		})
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

		// Snow base distance already retrieved from website
		baseDistance := h.getSnowBaseDistance(website)

		// Use the shared database fetching logic
		dbFetchedReadings, err = h.controller.fetchWeatherSpan(stationName, span, float64(baseDistance))
		if err != nil {
			log.Errorf("Error fetching weather span: %v", err)
			http.Error(w, "error fetching weather data", http.StatusInternalServerError)
			return
		}

		spanReadings := h.transformSpanReadings(&dbFetchedReadings)

		// Calculate cache duration based on data granularity
		cacheMaxAge := 60 // Default to 60 seconds
		if len(dbFetchedReadings) >= 2 {
			// Calculate the time difference between first two readings
			timeDiff := dbFetchedReadings[1].Bucket.Sub(dbFetchedReadings[0].Bucket)
			// Set cache to 1 second less than the data interval
			cacheSeconds := int(timeDiff.Seconds()) - 1
			if cacheSeconds > 0 {
				cacheMaxAge = cacheSeconds
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", cacheMaxAge))
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
	// Check if any website is configured
	website := h.getWebsiteFromContext(req)
	if website == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		h.formatter.WriteResponse(w, req, map[string]string{
			"error": "No weather websites configured",
			"message": "Weather data is not available until at least one weather website is configured",
		}, nil)
		return
	}

	if h.controller.DBEnabled {
		var dbFetchedReadings []types.BucketReading

		// Support both station_id (preferred) and station (legacy) parameters
		stationIDStr := req.URL.Query().Get("station_id")
		stationName := req.URL.Query().Get("station")

		// Convert station_id to station name if provided
		if stationIDStr != "" {
			stationID, err := strconv.Atoi(stationIDStr)
			if err != nil {
				http.Error(w, "invalid station_id", http.StatusBadRequest)
				return
			}
			// Find the station name from ID
			found := false
			for _, device := range h.controller.Devices {
				if device.ID == stationID {
					stationName = device.Name
					found = true
					break
				}
			}
			if !found {
				http.Error(w, "station not found", http.StatusNotFound)
				return
			}
		} else if stationName != "" && !h.validateStationExists(stationName) {
			// Validate station name if provided
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}

		// Find primary device for the website
		primaryDevice := h.getPrimaryDeviceForWebsite(website)

		// Determine which station to query
		queryStation := stationName
		if queryStation == "" {
			queryStation = primaryDevice
		}

		// Get base distance for snow depth calculation
		baseDistance := h.getSnowBaseDistance(website)

		// Use the shared database fetching logic
		fetchedReading, err := h.controller.fetchLatestReading(queryStation, float64(baseDistance))
		if err != nil {
			log.Errorf("Error fetching latest reading: %v", err)
			http.Error(w, "error fetching weather data", http.StatusInternalServerError)
			return
		}
		dbFetchedReadings = []types.BucketReading{*fetchedReading}

		latestReading := h.transformLatestReadings(&dbFetchedReadings)

		// Add total rainfall for the day using the shared optimized calculation
		if len(dbFetchedReadings) > 0 {
			stationName := dbFetchedReadings[0].StationName
			// Create a temporary database.Client wrapper for the CalculateDailyRainfall function
			dbClient := &database.Client{DB: h.controller.DB}
			calculatedDayRain := controllers.CalculateDailyRainfall(dbClient, stationName)
			latestReading.RainfallDay = calculatedDayRain
		}

		// Calculate rainfall totals using summary table with recent data
		if len(dbFetchedReadings) > 0 {
			stationName := dbFetchedReadings[0].StationName

			// Use the optimized function that combines summary + recent rain
			type RainfallPeriods struct {
				Rain24h float32 `gorm:"column:rain_24h"`
				Rain48h float32 `gorm:"column:rain_48h"`
				Rain72h float32 `gorm:"column:rain_72h"`
			}
			var rainfallPeriods RainfallPeriods
			
			// This query uses the pre-calculated summary and adds recent rain since last update
			err := h.controller.DB.Raw(`
				SELECT * FROM get_rainfall_with_recent(?)
			`, stationName).Scan(&rainfallPeriods).Error
			
			if err != nil {
				// Fallback to direct calculation if summary is not available
				log.Warnf("Failed to get rainfall from summary, falling back to direct calculation: %v", err)
				h.controller.DB.Raw(`
					SELECT 
						COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '24 hours' THEN period_rain END), 0) as rain_24h,
						COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '48 hours' THEN period_rain END), 0) as rain_48h,
						COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '72 hours' THEN period_rain END), 0) as rain_72h
					FROM weather_5m 
					WHERE stationname = ? AND bucket >= NOW() - INTERVAL '72 hours'
				`, stationName).Scan(&rainfallPeriods)
			}
			
			latestReading.Rainfall24h = rainfallPeriods.Rain24h
			latestReading.Rainfall48h = rainfallPeriods.Rain48h
			latestReading.Rainfall72h = rainfallPeriods.Rain72h

			// Storm rainfall total using existing function
			type StormRainResult struct {
				StormStart    *time.Time `gorm:"column:storm_start"`
				StormEnd      *time.Time `gorm:"column:storm_end"`
				TotalRainfall float32    `gorm:"column:total_rainfall"`
			}
			var stormResult StormRainResult
			err = h.controller.DB.Raw("SELECT * FROM calculate_storm_rainfall(?) LIMIT 1", stationName).Scan(&stormResult).Error
			if err != nil {
				log.Errorf("error getting storm rainfall from DB: %v", err)
			} else {
				latestReading.RainfallStorm = stormResult.TotalRainfall
			}
		}

		// Calculate wind gust from last 10 minutes
		if len(dbFetchedReadings) > 0 {
			type WindGustResult struct {
				WindGust float32
			}
			var windGustResult WindGustResult
			query := "SELECT calculate_wind_gust(?) AS wind_gust"
			err = h.controller.DB.Raw(query, dbFetchedReadings[0].StationName).Scan(&windGustResult).Error
			if err != nil {
				log.Errorf("error getting wind gust from DB: %v", err)
			} else {
				latestReading.WindGust = windGustResult.WindGust
			}
		}

		headers := map[string]string{
			"Cache-Control": "no-cache, no-store, must-revalidate",
		}
		err = h.formatter.WriteResponse(w, req, latestReading, headers)
		if err != nil {
			log.Error("error encoding latest weather readings:", err)
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
	if website == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		h.formatter.WriteResponse(w, req, map[string]string{
			"error": "No weather websites configured",
			"message": "Snow data is not available until at least one weather website is configured",
		}, nil)
		return
	}
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

		// Support both station_id (preferred) and station (legacy) parameters
		stationIDStr := req.URL.Query().Get("station_id")
		stationName := req.URL.Query().Get("station")

		// Convert station_id to station name if provided
		if stationIDStr != "" {
			stationID, err := strconv.Atoi(stationIDStr)
			if err != nil {
				http.Error(w, "invalid station_id", http.StatusBadRequest)
				return
			}
			// Find the station name from ID
			found := false
			for _, device := range h.controller.Devices {
				if device.ID == stationID {
					stationName = device.Name
					found = true
					break
				}
			}
			if !found {
				http.Error(w, "station not found", http.StatusNotFound)
				return
			}
		} else if stationName != "" && !h.validateStationExists(stationName) {
			// Validate station name if provided
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}

		// Get snow base distance from the snow device (needed for calculations)
		snowBaseDistance := h.getSnowBaseDistance(website)

		// Always use the snow device name for querying, unless a specific station is requested
		queryStation := website.SnowDeviceName
		if stationName != "" {
			queryStation = stationName
		}

		log.Debugf("GetSnowLatest: querying station='%s', snowBaseDistance=%.2f", queryStation, snowBaseDistance)
		// Query for latest reading from snow station, ensuring snowdistance is not null/zero
		h.controller.DB.Table("weather").
			Where("stationname = ?", queryStation).
			Where("snowdistance IS NOT NULL AND snowdistance > 0").
			Order("time DESC").
			Limit(1).
			Find(&dbFetchedReadings)

		log.Debugf("returned rows: %v", len(dbFetchedReadings))

		if len(dbFetchedReadings) > 0 {
			log.Debugf("latest snow reading: stationname='%s', snowdistance=%.2f, calculated_depth=%.2f",
				dbFetchedReadings[0].StationName,
				dbFetchedReadings[0].SnowDistance,
				mmToInches(snowBaseDistance - dbFetchedReadings[0].SnowDistance))
		}

		var result SnowDeltaResult

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
		snowfallRate := mmToInchesWithThreshold(result.Snowfall)

		// Check if we have any readings from the snow gauge
		var snowDepth float32 = 0.0
		if len(dbFetchedReadings) > 0 {
			snowDepth = mmToInchesWithThreshold(snowBaseDistance - dbFetchedReadings[0].SnowDistance)
		} else {
			log.Debugf("No readings available from snow device '%s' - returning zero values", website.SnowDeviceName)
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

		headers := map[string]string{
			"Cache-Control": "no-cache, no-store, must-revalidate",
		}
		err = h.formatter.WriteResponse(w, req, &snowReading, headers)
		if err != nil {
			log.Errorf("error encoding snowReading: %v", err)
			http.Error(w, "error encoding snow data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// GetForecast handles requests for forecast data
func (h *Handlers) GetForecast(w http.ResponseWriter, req *http.Request) {
	// Check if any website is configured
	website := h.getWebsiteFromContext(req)
	if website == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		h.formatter.WriteResponse(w, req, map[string]string{
			"error": "No weather websites configured",
			"message": "Forecast data is not available until at least one weather website is configured",
		}, nil)
		return
	}

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

		// Parse period parameter if provided
		periodStr := req.URL.Query().Get("period")
		var periodSpecified bool
		var period int
		if periodStr != "" {
			var err error
			period, err = strconv.Atoi(periodStr)
			if err != nil {
				log.Errorf("invalid period parameter: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				h.formatter.WriteResponse(w, req, map[string]string{"error": "Invalid period parameter - must be a number"}, nil)
				return
			}
			if period < 0 {
				log.Errorf("period must be non-negative, got %d", period)
				w.WriteHeader(http.StatusBadRequest)
				h.formatter.WriteResponse(w, req, map[string]string{"error": "period must be a non-negative number"}, nil)
				return
			}
			periodSpecified = true
		}

		// Support both station_id (preferred) and station (legacy) parameters
		stationIDStr := req.URL.Query().Get("station_id")
		stationName := req.URL.Query().Get("station")

		record := AerisWeatherForecastRecord{}

		var result *gorm.DB
		if stationIDStr != "" {
			// Use station_id if provided (preferred method)
			stationID, err := strconv.Atoi(stationIDStr)
			if err != nil {
				log.Errorf("invalid station_id: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid station_id parameter"})
				return
			}
			result = h.controller.DB.Where("station_id = ? AND forecast_span_hours = ?", stationID, span).First(&record)
		} else if stationName != "" {
			// Fall back to station name for backward compatibility
			// First, find the device ID for this station name
			devices, err := h.controller.configProvider.GetDevices()
			if err != nil {
				log.Errorf("error getting devices: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			
			var deviceID int
			found := false
			for _, device := range devices {
				if device.Name == stationName {
					deviceID = device.ID
					found = true
					break
				}
			}
			
			if !found {
				log.Errorf("station not found: %s", stationName)
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "Station not found"})
				return
			}
			
			result = h.controller.DB.Where("station_id = ? AND forecast_span_hours = ?", deviceID, span).First(&record)
		} else {
			// If no station specified, get the first available forecast for this span
			result = h.controller.DB.Where("forecast_span_hours = ?", span).First(&record)
		}
		if result.RowsAffected == 0 {
			log.Errorf("no forecast records found for span %v", span)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// If period is specified, return only that specific period
		dataBytes := record.Data.Bytes
		if periodSpecified {
			// Parse the JSON data
			var data []interface{}
			err := json.Unmarshal(dataBytes, &data)
			if err != nil {
				log.Errorf("error parsing forecast data: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				h.formatter.WriteResponse(w, req, map[string]string{"error": "Error processing forecast data"}, nil)
				return
			}

			// Check if period index is within bounds
			if period >= len(data) {
				log.Errorf("period %d exceeds available periods (0-%d)", period, len(data)-1)
				w.WriteHeader(http.StatusBadRequest)
				h.formatter.WriteResponse(w, req, map[string]string{
					"error": fmt.Sprintf("period %d exceeds available periods (valid range: 0-%d)", period, len(data)-1),
				}, nil)
				return
			}

			// Return only the requested period as a single element
			singlePeriod := data[period]
			
			// Re-encode the single period data
			singlePeriodBytes, err := json.Marshal(singlePeriod)
			if err != nil {
				log.Errorf("error encoding period data: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				h.formatter.WriteResponse(w, req, map[string]string{"error": "Error encoding period data"}, nil)
				return
			}
			dataBytes = singlePeriodBytes
		}

		wrapper := &responseformat.JSONWrapper{
			LastUpdated: record.UpdatedAt.String(),
		}
		err := h.formatter.WriteRawJSON(w, req, dataBytes, wrapper)
		if err != nil {
			log.Errorf("error encoding forecast: %v", err)
			http.Error(w, "error encoding forecast data", http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "database not enabled", http.StatusInternalServerError)
	}
}

// ServePortal serves the portal template for weather management portal websites
func (h *Handlers) ServePortal(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)

	// Check if website is configured
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "The REST server is running but no weather websites have been configured yet",
		})
		return
	}

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

	// Check if website is configured
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "The REST server is running but no weather websites have been configured yet",
		})
		return
	}

	// If this is a portal website, serve the portal instead
	if website.IsPortal {
		h.ServePortal(w, req)
		return
	}

	// Get the primary device for this website
	primaryDevice := h.getPrimaryDeviceConfigForWebsite(website)
	if primaryDevice == nil {
		http.Error(w, "No device configured for this website", http.StatusInternalServerError)
		return
	}

	view := htmltemplate.Must(htmltemplate.New("weather-station.html.tmpl").ParseFS(*h.controller.FS, "weather-station.html.tmpl"))

	// Create a template data structure with AboutStationHTML as safe HTML
	templateData := struct {
		StationName         string
		StationID           int
		PullFromDevice      string
		SnowEnabled         bool
		SnowDevice          string
		SnowBaseDistance    float32
		AirQualityEnabled   bool
		AirQualityDevice    string
		AirQualityDeviceID  int
		PageTitle           string
		AboutStationHTML    htmltemplate.HTML
		Version             string
		AerisWeatherEnabled bool
	}{
		StationName:         website.Name,
		StationID:           primaryDevice.ID,
		PullFromDevice:      primaryDevice.Name,
		SnowEnabled:         website.SnowEnabled,
		SnowDevice:          website.SnowDeviceName,
		SnowBaseDistance:    h.getSnowBaseDistance(website),
		AirQualityEnabled:   website.AirQualityEnabled,
		AirQualityDevice:    website.AirQualityDeviceName,
		AirQualityDeviceID:  h.getAirQualityDeviceID(website),
		PageTitle:           website.PageTitle,
		AboutStationHTML:    htmltemplate.HTML(website.AboutStationHTML),
		Version:             constants.Version,
		AerisWeatherEnabled: primaryDevice.AerisEnabled,
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
	
	// Check if website is configured
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "The REST server is running but no weather websites have been configured yet",
		})
		return
	}
	
	// Get the primary device for this website
	primaryDevice := h.getPrimaryDeviceConfigForWebsite(website)
	if primaryDevice == nil {
		http.Error(w, "No device configured for this website", http.StatusInternalServerError)
		return
	}

	view := template.Must(template.New("weather-app.js.tmpl").ParseFS(*h.controller.FS, "js/weather-app.js.tmpl"))

	// Create JS template data structure
	jsTemplateData := struct {
		StationName         string
		StationID           int
		PullFromDevice      string
		SnowEnabled         bool
		SnowDevice          string
		SnowBaseDistance    float32
		AirQualityEnabled   bool
		AirQualityDevice    string
		AirQualityDeviceID  int
		PageTitle           string
		AboutStationHTML    string
		AerisWeatherEnabled bool
	}{
		StationName:         website.Name,
		StationID:           primaryDevice.ID,
		PullFromDevice:      primaryDevice.Name,
		SnowEnabled:         website.SnowEnabled,
		SnowDevice:          website.SnowDeviceName,
		SnowBaseDistance:    h.getSnowBaseDistance(website),
		AirQualityEnabled:   website.AirQualityEnabled,
		AirQualityDevice:    website.AirQualityDeviceName,
		AirQualityDeviceID:  h.getAirQualityDeviceID(website),
		PageTitle:           website.PageTitle,
		AboutStationHTML:    website.AboutStationHTML,
		AerisWeatherEnabled: primaryDevice.AerisEnabled,
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

	// Check if website is configured
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "The REST server is running but no weather websites have been configured yet",
		})
		return
	}

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
				ID:        device.ID,
				Name:      device.Name,
				Type:      device.Type,
				Latitude:  device.Latitude,
				Longitude: device.Longitude,
				Enabled:   device.Enabled,
			}

			// Check if this device has an associated weather website
			if websiteData, exists := deviceToWebsite[device.ID]; exists && websiteData.Hostname != "" {
				// Determine if website has TLS configured
				hasTLS := (websiteData.TLSCertPath != "" && websiteData.TLSKeyPath != "")

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

// GetRemoteStations returns all registered remote stations
func (h *Handlers) GetRemoteStations(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)

	// Check if website is configured
	if website == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "No weather websites configured",
			"message": "The REST server is running but no weather websites have been configured yet",
		})
		return
	}

	// Only allow remote stations API access for portal websites
	if !website.IsPortal {
		http.Error(w, "remote stations API not available for this website", http.StatusForbidden)
		return
	}

	// Get remote stations from config provider
	cachedProvider, ok := h.controller.configProvider.(*config.CachedConfigProvider)
	if !ok {
		http.Error(w, "config provider does not support remote stations", http.StatusInternalServerError)
		return
	}

	remoteStations, err := cachedProvider.GetRemoteStations()
	if err != nil {
		log.Error("error loading remote stations:", err)
		http.Error(w, "error loading remote station data", http.StatusInternalServerError)
		return
	}

	// Transform to API response format
	type RemoteStationResponse struct {
		StationID   string    `json:"station_id"`
		StationName string    `json:"station_name"`
		StationType string    `json:"station_type"`
		LastSeen    time.Time `json:"last_seen"`
		Services    struct {
			APRS  bool `json:"aprs_enabled"`
			WU    bool `json:"wu_enabled"`
			Aeris bool `json:"aeris_enabled"`
			PWS   bool `json:"pws_enabled"`
		} `json:"services"`
		Status string `json:"status"` // "online", "offline", "stale"
	}

	response := make([]RemoteStationResponse, 0, len(remoteStations))
	now := time.Now()

	for _, station := range remoteStations {
		resp := RemoteStationResponse{
			StationID:   station.StationID,
			StationName: station.StationName,
			StationType: station.StationType,
			LastSeen:    station.LastSeen,
		}

		// Set service flags
		resp.Services.APRS = station.APRSEnabled
		resp.Services.WU = station.WUEnabled
		resp.Services.Aeris = station.AerisEnabled
		resp.Services.PWS = station.PWSEnabled

		// Determine status based on last seen time
		timeSinceLastSeen := now.Sub(station.LastSeen)
		if timeSinceLastSeen < 5*time.Minute {
			resp.Status = "online"
		} else if timeSinceLastSeen < 30*time.Minute {
			resp.Status = "stale"
		} else {
			resp.Status = "offline"
		}

		response = append(response, resp)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Error("error encoding remote stations to JSON:", err)
		http.Error(w, "error encoding remote stations", http.StatusInternalServerError)
		return
	}
}
