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
	if website, ok := req.Context().Value("website").(*config.WeatherWebsiteData); ok {
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

		vars := mux.Vars(req)
		span, err := time.ParseDuration(vars["span"])
		if err != nil {
			log.Errorf("invalid request: unable to parse duration: %v", vars["span"])
			http.Error(w, "error: invalid span duration", 400)
			return
		}

		spanStart := time.Now().Add(-span)
		// Get website from context and snow base distance
		website := h.getWebsiteFromContext(req)
		baseDistance := h.getSnowBaseDistance(website)

		switch {
		case span < 1*Day:
			if stationName != "" {
				h.controller.DB.Table("weather_1m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				h.controller.DB.Table("weather_1m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		case span >= 1*Day && span < 7*Day:
			if stationName != "" {
				h.controller.DB.Table("weather_5m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				h.controller.DB.Table("weather_5m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		case span >= 7*Day && span < 2*Month:
			if stationName != "" {
				h.controller.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				h.controller.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		default:
			if stationName != "" {
				h.controller.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				h.controller.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
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
		http.Error(w, "database not enabled", 500)
	}
}

// GetWeatherLatest handles requests for the latest weather data
func (h *Handlers) GetWeatherLatest(w http.ResponseWriter, req *http.Request) {
	if h.controller.DBEnabled {
		var dbFetchedReadings []types.BucketReading

		stationName := req.URL.Query().Get("station")

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

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		err := json.NewEncoder(w).Encode(latestReading)
		if err != nil {
			log.Error("error encoding latest weather readings to JSON:", err)
			return
		}
	} else {
		http.Error(w, "database not enabled", 500)
	}
}

// GetSnowLatest handles requests for the latest snow data
func (h *Handlers) GetSnowLatest(w http.ResponseWriter, req *http.Request) {
	// Get website from context and check if snow is enabled
	website := h.getWebsiteFromContext(req)
	if !website.SnowEnabled {
		http.Error(w, "snow data not enabled for this website", 404)
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
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow since midnight: %.2f mm\n", result.Snowfall)
		snowSinceMidnight := mmToInches(result.Snowfall)

		// Get the snowfall in the last 24 hours
		query = "SELECT get_new_snow_24h(?, ?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 24-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow in last 24h: %.2f mm\n", result.Snowfall)
		snowLast24 := mmToInches(result.Snowfall)

		// Get the snowfall in the last 72 hours
		query = "SELECT get_new_snow_72h(?, ?) AS snowfall"
		err = h.controller.DB.Raw(query, website.SnowDeviceName, snowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 72-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow in last 72h: %.2f mm\n", result.Snowfall)
		snowLast72 := mmToInches(result.Snowfall)

		snowReading := SnowReading{
			StationName: website.SnowDeviceName,
			SnowDepth:   mmToInches(snowBaseDistance - dbFetchedReadings[0].SnowDistance),
			SnowToday:   float32(snowSinceMidnight),
			SnowLast24:  float32(snowLast24),
			SnowLast72:  float32(snowLast72),
		}

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(&snowReading)
		if err != nil {
			log.Errorf("error marshalling snowReading: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}

		w.Write(jsonResponse)
	} else {
		http.Error(w, "database not enabled", 500)
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
			http.Error(w, "error: missing span duration", 400)
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
		http.Error(w, "database not enabled", 500)
	}
}

// ServeIndexTemplate serves the main HTML template
func (h *Handlers) ServeIndexTemplate(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)
	primaryDevice := h.getPrimaryDeviceForWebsite(website)

	view := htmltemplate.Must(htmltemplate.New("index.html.tmpl").ParseFS(*h.controller.FS, "index.html.tmpl"))

	// Create a template data structure with AboutStationHTML as safe HTML
	templateData := struct {
		StationName      string
		PullFromDevice   string
		SnowEnabled      bool
		SnowDevice       string
		SnowBaseDistance float32
		PageTitle        string
		AboutStationHTML htmltemplate.HTML // Convert to template.HTML to prevent escaping
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
		log.Error("error executing template:", err)
		return
	}
}

// ServeJS serves the JavaScript template
func (h *Handlers) ServeJS(w http.ResponseWriter, req *http.Request) {
	// Get website from context
	website := h.getWebsiteFromContext(req)
	primaryDevice := h.getPrimaryDeviceForWebsite(website)

	view := template.Must(template.New("remoteweather.js.tmpl").ParseFS(*h.controller.FS, "remoteweather.js.tmpl"))

	// Create JS template data structure compatible with the new website system
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
		log.Error("error executing template:", err)
		return
	}
}
