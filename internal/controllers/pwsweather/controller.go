// Package pwsweather provides integration with PWS Weather service for uploading weather data.
package pwsweather

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// PWSWeatherPayload represents the JSON payload for PWS Weather API
type PWSWeatherPayload struct {
	SoftwareType   string  `json:"softwareType"`
	DateTime       string  `json:"dateTime"`
	Metric         bool    `json:"metric"`
	Temp           float64 `json:"temp"`
	DewPt          float64 `json:"dewpt,omitempty"`
	Humidity       int     `json:"humidity"`
	Barometer      float64 `json:"barometer"`
	WindSpeed      int     `json:"windSpeed"`
	WindGust       int     `json:"windGust"`
	WindDir        int     `json:"windDir"`
	RainRate       float64 `json:"rainRate,omitempty"`
	RainTotal      float64 `json:"rainTotal"`
	SolarRadiation float64 `json:"solarRadiation,omitempty"`
	UV             float64 `json:"uv,omitempty"`
}

// PWSWeatherController holds our connection along with some mutexes for operation
type PWSWeatherController struct {
	*controllers.WeatherServiceController
	PWSWeatherConfig config.PWSWeatherData
}

func NewPWSWeatherController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, pwc config.PWSWeatherData, logger *zap.SugaredLogger) (*PWSWeatherController, error) {
	// Create base weather service controller
	base, err := controllers.NewWeatherServiceController(ctx, wg, configProvider, logger)
	if err != nil {
		return nil, err
	}

	// Validate PWS Weather specific configuration
	serviceConfig := controllers.WeatherServiceConfig{
		ServiceName:    "PWS Weather",
		StationID:      pwc.StationID,
		APIKey:         pwc.APIKey,
		PullFromDevice: pwc.PullFromDevice,
	}

	if err := controllers.ValidateWeatherServiceConfig(serviceConfig); err != nil {
		return nil, err
	}

	// Set defaults
	if pwc.APIEndpoint == "" {
		pwc.APIEndpoint = "https://pwsupdate.pwsweather.com/api/v1/submitwx"
	}
	if pwc.UploadInterval == "" {
		pwc.UploadInterval = "60"
	}

	// Validate pull-from-device exists
	if !base.DB.ValidatePullFromStation(pwc.PullFromDevice) {
		return nil, fmt.Errorf("pull-from-device %v is not a valid station name", pwc.PullFromDevice)
	}

	return &PWSWeatherController{
		WeatherServiceController: base,
		PWSWeatherConfig:         pwc,
	}, nil
}

func (p *PWSWeatherController) StartController() error {
	log.Info("Starting PWS Weather controller...")
	go p.sendPeriodicReports()
	return nil
}

func (p *PWSWeatherController) sendPeriodicReports() {
	config := controllers.WeatherServiceConfig{
		ServiceName:    "PWS Weather",
		StationID:      p.PWSWeatherConfig.StationID,
		APIKey:         p.PWSWeatherConfig.APIKey,
		APIEndpoint:    p.PWSWeatherConfig.APIEndpoint,
		UploadInterval: p.PWSWeatherConfig.UploadInterval,
		PullFromDevice: p.PWSWeatherConfig.PullFromDevice,
	}

	p.StartPeriodicReports(config, p.sendReadingsToPWSWeather)
}

func (p *PWSWeatherController) sendReadingsToPWSWeather(r *database.FetchedBucketReading) error {
	if err := controllers.ValidateReading(r); err != nil {
		return err
	}

	// Create the JSON payload
	payload := PWSWeatherPayload{
		SoftwareType:   fmt.Sprintf("RemoteWeather v%s", constants.Version),
		DateTime:       time.Now().UTC().Format(time.RFC3339),
		Metric:         false, // PWS Weather expects imperial units
		Temp:           float64(r.OutTemp),
		Humidity:       int(r.OutHumidity),
		Barometer:      float64(r.Barometer),
		WindSpeed:      int(r.WindSpeed),
		WindGust:       int(r.MaxWindSpeed),
		WindDir:        int(r.WindDir),
		RainTotal:      float64(r.DayRain),
		SolarRadiation: float64(r.SolarWatts),
	}

	// Convert to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling PWS Weather payload: %v", err)
	}

	// Create the request with authentication in URL
	authURL := fmt.Sprintf("%s?ID=%s&PASSWORD=%s", 
		p.PWSWeatherConfig.APIEndpoint,
		url.QueryEscape(p.PWSWeatherConfig.StationID),
		url.QueryEscape(p.PWSWeatherConfig.APIKey))

	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating PWS Weather request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Use the HTTP client from the base controller
	log.Debugf("Making POST request to PWS Weather: %s", p.PWSWeatherConfig.APIEndpoint)
	log.Debugf("Payload: %s", string(jsonData))

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to PWS Weather: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading PWS Weather response: %v", err)
	}

	log.Debugf("PWS Weather response status: %d, body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("PWS Weather API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
