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
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	DB             *database.Client
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

type AerisAlertsResponse struct {
	Success          bool              `json:"success"`
	Error            string            `json:"error"`
	AerisAlertsData  []AerisAlertData  `json:"response"`
}

type AerisAlertData struct {
	ID             string                 `json:"id"`
	Loc            AlertLocation          `json:"loc"`
	DataSource     string                 `json:"dataSource"`
	Details        AlertDetails           `json:"details"`
	Timestamps     AlertTimestamps        `json:"timestamps"`
	Poly           string                 `json:"poly"`
	GeoPoly        interface{}            `json:"geoPoly"`
	Includes       AlertIncludes          `json:"includes"`
	Place          AlertPlace             `json:"place"`
	Profile        AlertProfile           `json:"profile"`
	Active         bool                   `json:"active"`
	LocalLanguages []AlertLocalLanguage   `json:"localLanguages"`
}

type AlertLocation struct {
	Long float64 `json:"long"`
	Lat  float64 `json:"lat"`
}

type AlertDetails struct {
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	Loc       string  `json:"loc"`
	Emergency bool    `json:"emergency"`
	Priority  float64 `json:"priority"`
	Color     string  `json:"color"`
	Cat       string  `json:"cat"`
	Body      string  `json:"body"`
	BodyFull  string  `json:"bodyFull"`
}

type AlertTimestamps struct {
	Issued     int64  `json:"issued"`
	IssuedISO  string `json:"issuedISO"`
	Begins     int64  `json:"begins"`
	BeginsISO  string `json:"beginsISO"`
	Expires    int64  `json:"expires"`
	ExpiresISO string `json:"expiresISO"`
	Updated    int64  `json:"updated"`
	UpdatedISO string `json:"updateISO"`
	Added      int64  `json:"added"`
	AddedISO   string `json:"addedISO"`
	Created    int64  `json:"created"`
	CreatedISO string `json:"createdISO"`
}

type AlertIncludes struct {
	FIPS     []string `json:"fips"`
	Counties []string `json:"counties"`
	WXZones  []string `json:"wxzones"`
	Zipcodes []string `json:"zipcodes"`
}

type AlertPlace struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type AlertProfile struct {
	TZ           string `json:"tz"`
	IsSmallPoly  bool   `json:"isSmallPoly"`
}

type AlertLocalLanguage struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Body     string `json:"body"`
}

type AerisWeatherForecastRecord struct {
	gorm.Model

	StationID         int          `gorm:"uniqueIndex:idx_station_span,not null"`
	ForecastSpanHours int16        `gorm:"uniqueIndex:idx_station_span,not null"`
	Location          string       `gorm:"not null"`
	Data              pgtype.JSONB `gorm:"type:jsonb;default:'[]';not null"`
}

func (AerisWeatherForecastRecord) TableName() string {
	return "aeris_weather_forecasts"
}

type AerisWeatherAlertRecord struct {
	gorm.Model

	StationID int          `gorm:"index:idx_station_alerts,not null"`
	AlertID   string       `gorm:"uniqueIndex,not null"`
	Location  string       `gorm:"not null"`
	IssuedAt  *time.Time   `gorm:"index"`
	BeginsAt  *time.Time   `gorm:"index"`
	ExpiresAt *time.Time   `gorm:"index"`
	Name      string       `gorm:"type:text"`
	Color     string       `gorm:"type:text"`
	Body      string       `gorm:"type:text"`
	BodyFull  string       `gorm:"type:text"`
	Data      pgtype.JSONB `gorm:"type:jsonb;not null"`
}

func (AerisWeatherAlertRecord) TableName() string {
	return "aeris_weather_alerts"
}

func NewAerisWeatherController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, logger *zap.SugaredLogger) (*AerisWeatherController, error) {
	a := AerisWeatherController{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		logger:         logger,
	}

	// Validate TimescaleDB configuration
	if err := controllers.ValidateTimescaleDBConfig(configProvider, "Aeris Weather"); err != nil {
		return &AerisWeatherController{}, err
	}

	// Default API endpoint will be used from device config or fallback to default

	// Check if we have at least one device with Aeris enabled
	devices, err := configProvider.GetDevices()
	if err != nil {
		return &AerisWeatherController{}, fmt.Errorf("error loading device configurations: %v", err)
	}

	// Check if any device has Aeris enabled
	hasAerisDevice := false
	for _, device := range devices {
		if device.AerisEnabled && device.AerisAPIClientID != "" && device.AerisAPIClientSecret != "" &&
			device.Latitude != 0 && device.Longitude != 0 {
			hasAerisDevice = true
			break
		}
	}

	if !hasAerisDevice {
		log.Info("No Aeris Weather enabled devices found - controller will start but remain idle")
		// Continue initialization but controller will do nothing until devices are configured
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
	
	// Get all Aeris-enabled devices
	devices, err := a.configProvider.GetDevices()
	if err != nil {
		return fmt.Errorf("error getting devices: %v", err)
	}

	// Count Aeris-enabled devices
	enabledCount := 0
	for _, device := range devices {
		if device.AerisEnabled && device.AerisAPIClientID != "" && device.AerisAPIClientSecret != "" &&
			device.Latitude != 0 && device.Longitude != 0 {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		log.Info("No Aeris Weather enabled devices found")
		return nil
	}

	log.Infof("Found %d Aeris Weather enabled device(s)", enabledCount)

	// Start forecast fetching for each Aeris-enabled device
	for _, device := range devices {
		if device.AerisEnabled && device.AerisAPIClientID != "" && device.AerisAPIClientSecret != "" &&
			device.Latitude != 0 && device.Longitude != 0 {
			log.Infof("Starting Aeris Weather forecast fetching for device: %s (Location: %.6f,%.6f)",
				device.Name, device.Latitude, device.Longitude)

			// Create a copy for the closure
			deviceCopy := device

			// Start a refresh of the weekly forecast
			go a.refreshForecastPeriodically(deviceCopy, 10, 24)
			// Start a refresh of the hourly forecast
			go a.refreshForecastPeriodically(deviceCopy, 24, 1)
		}
	}

	// Start alerts fetching for each Aeris-enabled device
	for _, device := range devices {
		if device.AerisEnabled && device.AerisAPIClientID != "" && device.AerisAPIClientSecret != "" &&
			device.Latitude != 0 && device.Longitude != 0 {
			log.Infof("Starting Xweather alerts fetching for device: %s (Location: %.6f,%.6f)",
				device.Name, device.Latitude, device.Longitude)

			// Create a copy for the closure
			deviceCopy := device

			// Start alerts refresh (every 15 minutes)
			go a.refreshAlertsPeriodically(deviceCopy)
		}
	}

	// Start cleanup task for expired alerts (runs once daily for all devices)
	go a.cleanupExpiredAlerts()

	return nil
}

func (a *AerisWeatherController) refreshForecastPeriodically(device config.DeviceData, numPeriods int16, periodHours int16) {
	a.wg.Add(1)
	defer a.wg.Done()

	// time.Ticker's only begin to fire *after* the interval has elapsed.  Since we're dealing with
	// very long intervals, we will fire the fetcher now, before we start the ticker.
	log.Debugf("Starting initial forecast fetch for device %s: %d periods of %d hours", device.Name, numPeriods, periodHours)
	forecast, err := a.fetchAndStoreForecast(device, numPeriods, periodHours)
	if err != nil {
		log.Errorf("error fetching forecast from Aeris Weather for device %s: %v", device.Name, err)
	} else {
		// Only save to database if fetch was successful
		log.Debugf("Attempting to save forecast to database for device %s, span %d hours", device.Name, numPeriods*periodHours)
		// Create or update the forecast record
		locationStr := fmt.Sprintf("%.6f,%.6f", device.Latitude, device.Longitude)
		// Upsert the forecast record
		var existingRecord AerisWeatherForecastRecord
		err = a.DB.DB.Where("station_id = ? AND forecast_span_hours = ?", device.ID, numPeriods*periodHours).
			First(&existingRecord).Error
		
		if err == gorm.ErrRecordNotFound {
			// Create new record
			newRecord := AerisWeatherForecastRecord{
				StationID:         device.ID,
				ForecastSpanHours: numPeriods * periodHours,
				Location:          locationStr,
				Data:              forecast.Data,
			}
			err = a.DB.DB.Create(&newRecord).Error
		} else if err == nil {
			// Update existing record
			existingRecord.Data = forecast.Data
			existingRecord.Location = locationStr // Update location in case it changed
			err = a.DB.DB.Save(&existingRecord).Error
		}
		if err != nil {
			log.Errorf("error saving forecast to database for device %s: %v", device.Name, err)
		} else {
			log.Debugf("Successfully saved forecast to database for device %s, span %d hours", device.Name, numPeriods*periodHours)
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

	log.Infof("Starting Aeris Weather fetcher for device %s: %v hours, every %v minutes", 
		device.Name, numPeriods*periodHours, refreshInterval.Minutes())

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Infof("Updating forecast from Aeris Weather for device %s...", device.Name)
			forecast, err := a.fetchAndStoreForecast(device, numPeriods, periodHours)
			if err != nil {
				log.Errorf("error fetching forecast from Aeris Weather for device %s: %v", device.Name, err)
			} else {
				// Only save to database if fetch was successful
				log.Debugf("Attempting to save updated forecast to database for device %s, span %d hours", device.Name, numPeriods*periodHours)
				// Create or update the forecast record
				locationStr := fmt.Sprintf("%.6f,%.6f", device.Latitude, device.Longitude)
				// Upsert the forecast record
				var existingRecord AerisWeatherForecastRecord
				err = a.DB.DB.Where("station_id = ? AND forecast_span_hours = ?", device.ID, numPeriods*periodHours).
					First(&existingRecord).Error
				
				if err == gorm.ErrRecordNotFound {
					// Create new record
					newRecord := AerisWeatherForecastRecord{
						StationID:         device.ID,
						ForecastSpanHours: numPeriods * periodHours,
						Location:          locationStr,
						Data:              forecast.Data,
					}
					err = a.DB.DB.Create(&newRecord).Error
				} else if err == nil {
					// Update existing record
					existingRecord.Data = forecast.Data
					existingRecord.Location = locationStr // Update location in case it changed
					err = a.DB.DB.Save(&existingRecord).Error
				}
				if err != nil {
					log.Errorf("error saving forecast to database for device %s: %v", device.Name, err)
				} else {
					log.Debugf("Successfully saved updated forecast to database for device %s, span %d hours", device.Name, numPeriods*periodHours)
				}
			}

		case <-a.ctx.Done():
			return
		}
	}

}

func (a *AerisWeatherController) fetchAndStoreForecast(device config.DeviceData, numPeriods int16, periodHours int16) (*AerisWeatherForecastRecord, error) {
	v := url.Values{}

	// Add authentication from device
	v.Set("client_id", device.AerisAPIClientID)
	v.Set("client_secret", device.AerisAPIClientSecret)

	v.Set("filter", fmt.Sprintf("%vh", strconv.FormatInt(int64(periodHours), 10)))
	v.Set("limit", strconv.FormatInt(int64(numPeriods), 10))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Format coordinates as "latitude,longitude" for Aeris Weather API
	location := fmt.Sprintf("%.6f,%.6f", device.Latitude, device.Longitude)
	
	// Use device's API endpoint or default
	apiEndpoint := device.AerisAPIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "https://data.api.xweather.com"
	}
	url := fmt.Sprint(apiEndpoint + "/forecasts/" + location + "?" + v.Encode())
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

	// The request was succesful, so we need to add station ID, timespan and location information that will be stored
	// along side the forecast data in the database.  Station ID and span hours constitute a composite unique key
	// for the table.  Only one combination of station ID + span hours will be permitted.
	locationStr := fmt.Sprintf("%.6f,%.6f", device.Latitude, device.Longitude)
	record := AerisWeatherForecastRecord{
		StationID:         device.ID,
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

	err = a.DB.DB.AutoMigrate(AerisWeatherAlertRecord{})
	if err != nil {
		return fmt.Errorf("error creating or migrating Aeris alerts database table: %v", err)
	}

	return nil
}

// fetchAndStoreAlerts fetches current alerts from Xweather API and stores them in the database
func (a *AerisWeatherController) fetchAndStoreAlerts(device config.DeviceData) error {
	v := url.Values{}

	// Add authentication from device
	v.Set("client_id", device.AerisAPIClientID)
	v.Set("client_secret", device.AerisAPIClientSecret)

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Format coordinates as "latitude,longitude" for Xweather API
	location := fmt.Sprintf("%.6f,%.6f", device.Latitude, device.Longitude)
	locationStr := location

	// Use device's API endpoint or default
	apiEndpoint := device.AerisAPIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "https://data.api.xweather.com"
	}

	alertsURL := fmt.Sprintf("%s/alerts/%s?%s", apiEndpoint, location, v.Encode())
	req, err := http.NewRequest("GET", alertsURL, nil)
	if err != nil {
		return fmt.Errorf("error creating Xweather alerts API HTTP request: %v", err)
	}

	log.Debugf("Making request to Xweather alerts API: %v", alertsURL)
	req = req.WithContext(a.ctx)
	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("HTTP request failed: %v", err)
		return fmt.Errorf("error making request to Xweather alerts API: %v", err)
	}
	defer resp.Body.Close()

	log.Debugf("Xweather alerts API responded with status: %s", resp.Status)

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Failed to read response body: %v", err)
		return fmt.Errorf("error reading response body: %v", err)
	}

	log.Debugf("Xweather alerts API response body: %s", string(bodyBytes))

	response := &AerisAlertsResponse{}
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))

	err = decoder.Decode(response)
	if err != nil {
		return fmt.Errorf("unable to decode Xweather alerts API response: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("bad response from Xweather alerts server: %+v", response)
	}

	// Process each alert from the response
	for _, alertData := range response.AerisAlertsData {
		// Parse timestamps
		var issuedAt, beginsAt, expiresAt *time.Time

		if alertData.Timestamps.Issued > 0 {
			t := time.Unix(alertData.Timestamps.Issued, 0)
			issuedAt = &t
		}
		if alertData.Timestamps.Begins > 0 {
			t := time.Unix(alertData.Timestamps.Begins, 0)
			beginsAt = &t
		}
		if alertData.Timestamps.Expires > 0 {
			t := time.Unix(alertData.Timestamps.Expires, 0)
			expiresAt = &t
		}

		// Marshal complete alert data to JSON for storage
		alertJSON, err := json.Marshal(alertData)
		if err != nil {
			log.Errorf("error marshaling alert data to JSON for alert %s: %v", alertData.ID, err)
			continue
		}

		// Check if this alert already exists
		var existingRecord AerisWeatherAlertRecord
		err = a.DB.DB.Where("alert_id = ?", alertData.ID).First(&existingRecord).Error

		if err == gorm.ErrRecordNotFound {
			// Create new alert record
			newRecord := AerisWeatherAlertRecord{
				StationID: device.ID,
				AlertID:   alertData.ID,
				Location:  locationStr,
				IssuedAt:  issuedAt,
				BeginsAt:  beginsAt,
				ExpiresAt: expiresAt,
				Name:      alertData.Details.Name,
				Color:     alertData.Details.Color,
				Body:      alertData.Details.Body,
				BodyFull:  alertData.Details.BodyFull,
			}
			newRecord.Data.Set(alertJSON)

			err = a.DB.DB.Create(&newRecord).Error
			if err != nil {
				log.Errorf("error creating alert record for %s: %v", alertData.ID, err)
			} else {
				log.Debugf("Created new alert record: %s for device %s", alertData.ID, device.Name)
			}
		} else if err == nil {
			// Update existing alert record
			existingRecord.IssuedAt = issuedAt
			existingRecord.BeginsAt = beginsAt
			existingRecord.ExpiresAt = expiresAt
			existingRecord.Name = alertData.Details.Name
			existingRecord.Color = alertData.Details.Color
			existingRecord.Body = alertData.Details.Body
			existingRecord.BodyFull = alertData.Details.BodyFull
			existingRecord.Location = locationStr
			existingRecord.Data.Set(alertJSON)

			err = a.DB.DB.Save(&existingRecord).Error
			if err != nil {
				log.Errorf("error updating alert record for %s: %v", alertData.ID, err)
			} else {
				log.Debugf("Updated alert record: %s for device %s", alertData.ID, device.Name)
			}
		} else {
			log.Errorf("error checking for existing alert %s: %v", alertData.ID, err)
		}
	}

	log.Debugf("Successfully processed %d alerts for device %s", len(response.AerisAlertsData), device.Name)
	return nil
}

// refreshAlertsPeriodically fetches and stores alerts on a 15-minute interval
func (a *AerisWeatherController) refreshAlertsPeriodically(device config.DeviceData) {
	a.wg.Add(1)
	defer a.wg.Done()

	// Fetch immediately on startup
	log.Infof("Fetching initial alerts for device %s...", device.Name)
	err := a.fetchAndStoreAlerts(device)
	if err != nil {
		log.Errorf("error fetching alerts from Xweather for device %s: %v", device.Name, err)
	}

	// Refresh every 15 minutes
	refreshInterval := 15 * time.Minute
	log.Infof("Starting Xweather alerts fetcher for device %s: every %v minutes", device.Name, refreshInterval.Minutes())

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Infof("Updating alerts from Xweather for device %s...", device.Name)
			err := a.fetchAndStoreAlerts(device)
			if err != nil {
				log.Errorf("error fetching alerts from Xweather for device %s: %v", device.Name, err)
			}

		case <-a.ctx.Done():
			return
		}
	}
}

// cleanupExpiredAlerts removes alerts that expired more than 24 hours ago
func (a *AerisWeatherController) cleanupExpiredAlerts() {
	a.wg.Add(1)
	defer a.wg.Done()

	// Run cleanup immediately on startup
	log.Info("Running initial expired alerts cleanup...")
	cutoffTime := time.Now().Add(-24 * time.Hour)
	result := a.DB.DB.Where("expires_at < ?", cutoffTime).Delete(&AerisWeatherAlertRecord{})
	if result.Error != nil {
		log.Errorf("error cleaning up expired alerts: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Infof("Cleaned up %d expired alerts", result.RowsAffected)
	}

	// Run cleanup once per day
	cleanupInterval := 24 * time.Hour
	log.Infof("Starting expired alerts cleanup task: every %v hours", cleanupInterval.Hours())

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Info("Running expired alerts cleanup...")
			cutoffTime := time.Now().Add(-24 * time.Hour)
			result := a.DB.DB.Where("expires_at < ?", cutoffTime).Delete(&AerisWeatherAlertRecord{})
			if result.Error != nil {
				log.Errorf("error cleaning up expired alerts: %v", result.Error)
			} else if result.RowsAffected > 0 {
				log.Infof("Cleaned up %d expired alerts", result.RowsAffected)
			}

		case <-a.ctx.Done():
			return
		}
	}
}
