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

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/jackc/pgtype"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AerisWeatherController holds our AerisWeather configuration
type AerisWeatherController struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	config             *types.Config
	AerisWeatherConfig types.AerisWeatherConfig
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

func NewAerisWeatherController(ctx context.Context, wg *sync.WaitGroup, c *types.Config, ac types.AerisWeatherConfig, logger *zap.SugaredLogger) (*AerisWeatherController, error) {
	a := AerisWeatherController{
		ctx:                ctx,
		wg:                 wg,
		config:             c,
		AerisWeatherConfig: ac,
		logger:             logger,
	}

	if a.config.Storage.TimescaleDB.ConnectionString == "" {
		return &AerisWeatherController{}, fmt.Errorf("TimescaleDB storage must be configured for the Aeris controller to function")
	}

	if a.AerisWeatherConfig.APIClientID == "" {
		return &AerisWeatherController{}, fmt.Errorf("API client id must be set (this is provided by Aeris Weather)")
	}

	if a.AerisWeatherConfig.APIClientSecret == "" {
		return &AerisWeatherController{}, fmt.Errorf("API client secret must be set (this is provided by Aeris Weather)")
	}

	if a.AerisWeatherConfig.APIEndpoint == "" {
		// Set a default API endpoint if not provided
		a.AerisWeatherConfig.APIEndpoint = "https://api.aerisapi.com"
	}

	if a.AerisWeatherConfig.Location == "" {
		return &AerisWeatherController{}, fmt.Errorf("forecast location must be set")
	}

	a.DB = database.NewClient(c, logger)

	// Connect to TimescaleDB for purposes of storing Aeris data for future client requests
	err := a.DB.ConnectToTimescaleDB()
	if err != nil {
		return &AerisWeatherController{}, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

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
	forecast, err := a.fetchAndStoreForecast(numPeriods, periodHours)
	if err != nil {
		log.Error("error fetching forecast from Aeris Weather:", err)
	} else {
		// Only save to database if fetch was successful
		err = a.DB.DB.Model(&AerisWeatherForecastRecord{}).Where("forecast_span_hours = ?", numPeriods*periodHours).Update("data", forecast.Data).Error
		if err != nil {
			log.Errorf("error saving forecast to database: %v", err)
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
				err = a.DB.DB.Model(&AerisWeatherForecastRecord{}).Where("forecast_span_hours = ?", numPeriods*periodHours).Update("data", forecast.Data).Error
				if err != nil {
					log.Errorf("error saving forecast to database: %v", err)
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

	url := fmt.Sprint(a.AerisWeatherConfig.APIEndpoint + "/forecasts/" + a.AerisWeatherConfig.Location + "?" + v.Encode())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("error creating Aeris Weather API HTTP request: %v", err)
	}

	log.Debugf("Making request to Aeris Weather: %v", url)
	req = req.WithContext(a.ctx)
	resp, err := client.Do(req)
	if err != nil {
		return &AerisWeatherForecastRecord{}, fmt.Errorf("error making request to Aeris Weather: %v", err)
	}

	response := &AerisWeatherForecastResponse{}

	decoder := json.NewDecoder(resp.Body)

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
	record := AerisWeatherForecastRecord{
		ForecastSpanHours: numPeriods * periodHours,
		Location:          a.AerisWeatherConfig.Location,
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
