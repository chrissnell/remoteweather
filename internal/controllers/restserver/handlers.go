package restserver

import (
	"encoding/json"
	htmltemplate "html/template"
	"net/http"
	"time"

	"text/template"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/gorilla/mux"
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
		baseDistance := h.controller.WeatherSiteConfig.SnowBaseDistance

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
		case span >= 1*Day && span <= 7*Day:
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
		case span > 7*Day:
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
		baseDistance := h.controller.WeatherSiteConfig.SnowBaseDistance

		if stationName != "" {
			h.controller.DB.Table("weather_1m").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > NOW() - INTERVAL '2 minutes'").
				Where("stationname = ?", stationName).
				Order("bucket DESC").
				Limit(1).
				Find(&dbFetchedReadings)
		} else {
			h.controller.DB.Table("weather_1m").
				Select("*, (? - snowdistance) AS snowdepth", baseDistance).
				Where("bucket > NOW() - INTERVAL '2 minutes'").
				Order("bucket DESC").
				Limit(1).
				Find(&dbFetchedReadings)
		}

		latestReading := h.transformLatestReadings(&dbFetchedReadings)

		// Add total rainfall for the day
		type Rainfall struct {
			TotalRain float32
		}

		var totalRainfall Rainfall
		h.controller.DB.Table("weather_1m").
			Select("MAX(dayrain) as total_rain").
			Where("DATE(bucket) = CURRENT_DATE").
			Where("stationname = ?", h.controller.WeatherSiteConfig.PullFromDevice).
			Scan(&totalRainfall)

		latestReading.RainfallDay = float32ToJSONNumber(totalRainfall.TotalRain)

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
	// Snow handler implementation would go here
	// For now, return a placeholder
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "snow endpoint not implemented"})
}

// GetForecast handles requests for forecast data
func (h *Handlers) GetForecast(w http.ResponseWriter, req *http.Request) {
	// Forecast handler implementation would go here
	// For now, return a placeholder
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "forecast endpoint not implemented"})
}

// ServeIndexTemplate serves the main HTML template
func (h *Handlers) ServeIndexTemplate(w http.ResponseWriter, req *http.Request) {
	view := htmltemplate.Must(htmltemplate.New("index.html.tmpl").ParseFS(*h.controller.FS, "index.html.tmpl"))

	w.Header().Set("Content-Type", "text/html")
	err := view.Execute(w, h.controller.WeatherSiteConfig)
	if err != nil {
		log.Error("error executing template:", err)
		return
	}
}

// ServeJS serves the JavaScript template
func (h *Handlers) ServeJS(w http.ResponseWriter, req *http.Request) {
	view := template.Must(template.New("remoteweather.js.tmpl").ParseFS(*h.controller.FS, "remoteweather.js.tmpl"))

	w.Header().Set("Content-Type", "text/javascript")
	err := view.Execute(w, h.controller.WeatherSiteConfig)
	if err != nil {
		log.Error("error executing template:", err)
		return
	}
}
