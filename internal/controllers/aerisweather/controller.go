// Package aerisweather provides integration with Aeris Weather API for forecast data.
package aerisweather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"bytes"
	"io"

	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/jackc/pgtype"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AerisWeatherController holds our AerisWeather configuration
type AerisWeatherController struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	configProvider     config.ConfigProvider
	AerisWeatherConfig config.AerisWeatherData
	logger             *zap.SugaredLogger
	DB                 *database.Client
}

type AerisWeatherForecastResponse struct {
	Success           bool                               `json:"success"`
	Error             string                             `json:"error"`
	AerisForecastData []AerisWeatherForecastResponseData `json:"response"`
}

type AerisWeatherForecastResponseData struct {
	Periods []AerisWeatherForecastPeriod `json:"periods"`
}

type AerisWeatherForecastPeriod struct {
	ForecastIntervalStart time.Time `json:"dateTimeISO" gorm:"not null"`
	MaxTempF              int16     `json:"maxTempF"`
	MinTempF              int16     `json:"minTempF"`
	AvgTempF              int16     `json:"avgTempF"`
	PrecipProbability     int16     `json:"pop"`
	PrecipInches          float32   `json:"precipIN"`
	IceInches             float32   `json:"iceaccumIN"`
	SnowInches            float32   `json:"snowIN"`
	MaxFeelsLike          int16     `json:"maxFeelslikeF"`
	MinFeelsLike          int16     `json:"minFeelslikeF"`
	WindSpeedMPH          int16     `json:"windSpeedMPH"`
	WindSpeedMax          int16     `json:"windSpeedMaxMPH"`
	WindDir               string    `json:"windDir"`
	WindDirDeg            uint16    `json:"windDirDEG"`
	Weather               string    `json:"weather"`
	WeatherCoded          string    `json:"weatherPrimaryCoded"`
	WeatherIcon           string    `json:"weatherIcon"`
	CompactWeather        string    `json:"compactWeather"`
}

type AerisWeatherForecastRecord struct {
	gorm.Model

	ForecastSpanHours int16        `gorm:"uniqueIndex:idx_location_span,not null"`
	Location          string       `gorm:"uniqueIndex:idx_location_span,not null"`
	Data              pgtype.JSONB `gorm:"type:jsonb;default:'[]';not null"`
}

func (AerisWeatherForecastRecord) TableName() string {
	return "aeris_weather_forecasts"
}

func NewAerisWeatherController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, ac config.AerisWeatherData, logger *zap.SugaredLogger) (*AerisWeatherController, error) {
	a := AerisWeatherController{
		ctx:                ctx,
		wg:                 wg,
		configProvider:     configProvider,
		AerisWeatherConfig: ac,
		logger:             logger,
	}

	// Validate TimescaleDB configuration
	if err := controllers.ValidateTimescaleDBConfig(configProvider, "Aeris Weather"); err != nil {
		return &AerisWeatherController{}, err
	}

	// Validate required fields
	fields := map[string]string{
		"API client ID":     a.AerisWeatherConfig.APIClientID,
		"API client secret": a.AerisWeatherConfig.APIClientSecret,
	}
	if err := controllers.ValidateRequiredFields(fields); err != nil {
		return &AerisWeatherController{}, err
	}

	// Set defaults
	if a.AerisWeatherConfig.APIEndpoint == "" {
		a.AerisWeatherConfig.APIEndpoint = "https://api.aerisapi.com"
	}

	if a.AerisWeatherConfig.Latitude == 0 || a.AerisWeatherConfig.Longitude == 0 {
		return &AerisWeatherController{}, fmt.Errorf("forecast latitude and longitude must be set")
	}

	// Setup database connection
	db, err := controllers.SetupDatabaseConnection(configProvider, logger)
	if err != nil {
		return &AerisWeatherController{}, err
	}
	a.DB = db

	err = a.CreateTables()
	if err != nil {
		return &AerisWeatherController{}, err
	}

	return &a, nil
}

func (a *AerisWeatherController) StartController() error {
	log.Info("Starting Aeris Weather controller...")
	a.wg.Add(1)
	defer a.wg.Done()

	// Start a refresh of the weekly forecast
	go a.refreshForecastPeriodically(7, 24)
	// Start a refresh of the hourly forecast
	go a.refreshForecastPeriodically(24, 1)
	return nil
}

func (a *AerisWeatherController) refreshForecastPeriodically(numPeriods int16, periodHours int16) {
	a.wg.Add(1)
	defer a.wg.Done()

	// time.Ticker's only begin to fire *after* the interval has elapsed.  Since we're dealing with
	// very long intervals, we will fire the fetcher now, before we start the ticker.
	log.Debugf("Starting initial forecast fetch for %d periods of %d hours", numPeriods, periodHours)
	forecast, err := a.fetchAndStoreForecast(numPeriods, periodHours)
	if err != nil {
		log.Error("error fetching forecast from Aeris Weather:", err)
	} else {
		// Only save to database if fetch was successful
		log.Debugf("Attempting to save forecast to database for span %d hours", numPeriods*periodHours)
		// Create or update the forecast record
		locationStr := fmt.Sprintf("%.6f,%.6f", a.AerisWeatherConfig.Latitude, a.AerisWeatherConfig.Longitude)
		err = a.DB.DB.Where("forecast_span_hours = ? AND location = ?", numPeriods*periodHours, locationStr).
			Assign(AerisWeatherForecastRecord{
				ForecastSpanHours: numPeriods * periodHours,
				Location:          locationStr,
				Data:              forecast.Data,
			}).
			FirstOrCreate(&AerisWeatherForecastRecord{}).Error
		if err != nil {
			log.Errorf("error saving forecast to database: %v", err)
		} else {
			log.Debugf("Successfully saved forecast to database for span %d hours", numPeriods*periodHours)
		}
	}

	// Convert periodHours into a time.Duration
	spanInterval, err := time.ParseDuration(fmt.Sprintf("%vh", periodHours))
	if err != nil {
		log.Errorf("error parsing Aeris Weather refresh interval:", err)
	}

	// We will refresh our forecasts four times in every period.
	// For example: for a daily forecast, we refresh every 6 hours.
	refreshInterval := spanInterval / 4

	log.Infof("Starting Aeris Weather fetcher for %v hours, every %v minutes", numPeriods*periodHours, refreshInterval.Minutes())

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Info("Updating forecast from Aeris Weather...")
			forecast, err := a.fetchAndStoreForecast(numPeriods, periodHours)
			if err != nil {
				log.Error("error fetching forecast from Aeris Weather:", err)
			} else {
				// Only save to database if fetch was successful
				log.Debugf("Attempting to save updated forecast to database for span %d hours", numPeriods*periodHours)
				// Create or update the forecast record
				locationStr := fmt.Sprintf("%.6f,%.6f", a.AerisWeatherConfig.Latitude, a.AerisWeatherConfig.Longitude)
				err = a.DB.DB.Where("forecast_span_hours = ? AND location = ?", numPeriods*periodHours, locationStr).
					Assign(AerisWeatherForecastRecord{
						ForecastSpanHours: numPeriods * periodHours,
						Location:          locationStr,
						Data:              forecast.Data,
					}).
					FirstOrCreate(&AerisWeatherForecastRecord{}).Error
				if err != nil {
					log.Errorf("error saving forecast to database: %v", err)
				} else {
					log.Debugf("Successfully saved updated forecast to database for span %d hours", numPeriods*periodHours)
				}
			}

		case <-a.ctx.Done():
			return
		}
	}

}

func (a *AerisWeatherController) fetchAndStoreForecast(numPeriods int16, periodHours int16) (*AerisWeatherForecastRecord, error) {
	v := url.Values{}

	// Add authentication
	v.Set("client_id", a.AerisWeatherConfig.APIClientID)
	v.Set("client_secret", a.AerisWeatherConfig.APIClientSecret)

	v.Set("filter", fmt.Sprintf("%vh", strconv.FormatInt(int64(periodHours), 10)))
	v.Set("limit", strconv.FormatInt(int64(numPeriods), 10))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Format coordinates as "latitude,longitude" for Aeris Weather API
	location := fmt.Sprintf("%.6f,%.6f", a.AerisWeatherConfig.Latitude, a.AerisWeatherConfig.Longitude)
	url := fmt.Sprint(a.AerisWeatherConfig.APIEndpoint + "/forecasts/" + location + "?" + v.Encode())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("error creating Aeris Weather API HTTP request: %v", err)
	}

	log.Debugf("Making request to Aeris Weather: %v", url)
	req = req.WithContext(a.ctx)
	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("HTTP request failed: %v", err)
		return &AerisWeatherForecastRecord{}, fmt.Errorf("error making request to Aeris Weather: %v", err)
	}
	defer resp.Body.Close()

	log.Debugf("Aeris Weather API responded with status: %s", resp.Status)

	// Read the response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Failed to read response body: %v", err)
		return &AerisWeatherForecastRecord{}, fmt.Errorf("error reading response body: %v", err)
	}

	log.Debugf("Aeris Weather API response body: %s", string(bodyBytes))

	response := &AerisWeatherForecastResponse{}

	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))

	err = decoder.Decode(response)
	if err != nil {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("unable to decode Aeris Weather API response: %v", err)
	}

	if !response.Success {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("bad response from Aeris Weather server %+v", response)
	}

	// This is a map of Aeris Weather weather codes to symbols from the Weather Icons font set
	// See:   https://www.aerisweather.com/support/docs/api/reference/weather-codes/
	//        https://erikflowers.github.io/weather-icons/
	var iconMap = map[string]string{
		"A":  "", // hail
		"BD": "", // blowing dust
		"BN": "", // blowing sand
		"BR": "", // mist
		"BS": "", // blowing snow
		"BY": "", // blowing spray
		"F":  "", // fog
		"FC": "", // funnel cloud
		"FR": "", // frost
		"H":  "", // haze
		"IC": "", // ice crystals
		"IF": "", // ice fog
		"IP": "", // ice pellets/sleet
		"K":  "", // smoke
		"L":  "", // drizzle
		"R":  "", // rain
		"RW": "", // rain showers
		"RS": "", // rain-snow mix
		"SI": "", // snow-sleet mix
		"WM": "", // wintry mix
		"S":  "", // snow
		"SW": "", // snow showers
		"T":  "", // thunderstorm
		"UP": "", // unknown precip
		"VA": "", // volcanic ash
		"WP": "", // waterspouts
		"ZF": "", // freezing fog
		"ZL": "", // freezing drizzle
		"ZR": "", // freezing rain
		"ZY": "", // freezing spray
		"CL": "", // clear
		"FW": "", // mostly sunny
		"SC": "", // partly cloudy
		"BK": "", // mostly cloudy
		"OV": "", // cloudy/overcast
	}

	// This is a map of Aeris Weather weather codes to compact descriptions of the weather
	// See:   https://www.aerisweather.com/support/docs/api/reference/weather-codes/
	var compactWeatherMap = map[string]string{
		"A":  "Hail",             // hail
		"BD": "Blowing Dust",     // blowing dust
		"BN": "Blowing Sand",     // blowing sand
		"BR": "Mist",             // mist
		"BS": "Blowing Snow",     // blowing snow
		"BY": "Blowing Spray",    // blowing spray
		"F":  "Fog",              // fog
		"FC": "Funnel Clouds",    // funnel cloud
		"FR": "Frost",            // frost
		"H":  "Haze",             // haze
		"IC": "Ice Crystals",     // ice crystals
		"IF": "Ice Fog",          // ice fog
		"IP": "Sleet",            // ice pellets/sleet
		"K":  "Smoke",            // smoke
		"L":  "Drizzle",          // drizzle
		"R":  "Rain",             // rain
		"RW": "Rain Showers",     // rain showers
		"RS": "Rain-Snow Mix",    // rain-snow mix
		"SI": "Snow-Sleet Mix",   // snow-sleet mix
		"WM": "Wintry Mix",       // wintry mix
		"S":  "Snow",             // snow
		"SW": "Snow Showers",     // snow showers
		"T":  "Thunderstorms",    // thunderstorm
		"UP": "Unknown Precip",   // unknown precip
		"VA": "Volcanic Ash",     // volcanic ash
		"WP": "Waterspouts",      // waterspouts
		"ZF": "Freezing Fog",     // freezing fog
		"ZL": "Freezing Drizzle", // freezing drizzle
		"ZR": "Freezing Rain",    // freezing rain
		"ZY": "Freezing Spray",   // freezing spray
		"CL": "Clear",            // clear
		"FW": "Mostly Sunny",     // mostly sunny
		"SC": "Partly Cloudy",    // partly cloudy
		"BK": "Mostly Cloudy",    // mostly cloudy
		"OV": "Cloudy",           // cloudy/overcast
	}

	// Add icons to the period data
	for k, p := range response.AerisForecastData[0].Periods {
		forecastParts := strings.Split(p.WeatherCoded, ":")
		if forecastParts[2] != "" {
			response.AerisForecastData[0].Periods[k].WeatherIcon = iconMap[forecastParts[2]]
			response.AerisForecastData[0].Periods[k].CompactWeather = compactWeatherMap[forecastParts[2]]
		} else {
			response.AerisForecastData[0].Periods[k].WeatherIcon = "?"
			response.AerisForecastData[0].Periods[k].CompactWeather = ""
		}
	}

	forecastPeriodsJSON, err := json.Marshal(response.AerisForecastData[0].Periods)
	if err != nil {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("could not marshall forecast periods to JSON: %v", err)
	}

	// The request was succesful, so we need to add timespan and location information that will be stored
	// along side the forecast data in the database.  Together, these constitute a composite primary key
	// for the table.  Only one combination of span hours + location will be permitted.
	locationStr := fmt.Sprintf("%.6f,%.6f", a.AerisWeatherConfig.Latitude, a.AerisWeatherConfig.Longitude)
	record := AerisWeatherForecastRecord{
		ForecastSpanHours: numPeriods * periodHours,
		Location:          locationStr,
	}
	record.Data.Set(forecastPeriodsJSON)

	return &record, nil
}

func (a *AerisWeatherController) CreateTables() error {
	err := a.DB.DB.AutoMigrate(AerisWeatherForecastRecord{})
	if err != nil {
		return fmt.Errorf("error creating or migrating Aeris forecast record database table: %v", err)
	}

	return nil
}
