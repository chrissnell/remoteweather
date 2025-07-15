package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// WeatherServiceController represents a generic weather service controller
type WeatherServiceController struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	logger         *zap.SugaredLogger
	DB             *database.Client
	httpClient     *http.Client
}

// WeatherServiceConfig holds common configuration for weather service controllers
type WeatherServiceConfig struct {
	ServiceName    string
	StationID      string
	APIKey         string
	APIEndpoint    string
	UploadInterval string
	PullFromDevice string
}

// NewWeatherServiceController creates a new weather service controller base
func NewWeatherServiceController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, logger *zap.SugaredLogger) (*WeatherServiceController, error) {
	// Validate TimescaleDB configuration
	if err := ValidateTimescaleDBConfig(configProvider, "weather service"); err != nil {
		return nil, err
	}

	// Set up database connection
	db, err := SetupDatabaseConnection(configProvider, logger)
	if err != nil {
		return nil, err
	}

	return &WeatherServiceController{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		logger:         logger,
		DB:             db,
		httpClient:     NewHTTPClient(5 * time.Second),
	}, nil
}

// ValidateWeatherServiceConfig validates common weather service configuration
func ValidateWeatherServiceConfig(wsc WeatherServiceConfig) error {
	fields := map[string]string{
		"station ID":       wsc.StationID,
		"API key":          wsc.APIKey,
		"pull-from-device": wsc.PullFromDevice,
	}

	return ValidateRequiredFields(fields)
}

// SetWeatherServiceDefaults sets default values for weather service configuration
func SetWeatherServiceDefaults(wsc *WeatherServiceConfig) {
	if wsc.UploadInterval == "" {
		wsc.UploadInterval = "60"
	}
}

// StartPeriodicReports starts periodic weather reports using the provided send function
func (wsc *WeatherServiceController) StartPeriodicReports(config WeatherServiceConfig, sendFunc func(*database.FetchedBucketReading) error) {
	wsc.wg.Add(1)
	defer wsc.wg.Done()

	submitInterval, err := time.ParseDuration(fmt.Sprintf("%vs", config.UploadInterval))
	if err != nil {
		wsc.logger.Errorf("error parsing duration: %v", err)
		return
	}

	ticker := time.NewTicker(submitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wsc.logger.Debugf("Sending reading to %s...", config.ServiceName)
			br, err := wsc.DB.GetReadingsFromTimescaleDB(config.PullFromDevice)
			if err != nil {
				wsc.logger.Errorf("error getting readings from TimescaleDB: %v", err)
				continue
			}
			wsc.logger.Debugf("readings fetched from TimescaleDB for %s: %+v", config.ServiceName, br)

			if err := sendFunc(&br); err != nil {
				wsc.logger.Errorf("error sending readings to %s: %v", config.ServiceName, err)
			}
		case <-wsc.ctx.Done():
			return
		}
	}
}

// SendHTTPRequest sends an HTTP GET request with URL-encoded parameters
func (wsc *WeatherServiceController) SendHTTPRequest(endpoint string, params url.Values) error {
	// Build the full URL with query parameters
	fullURL := fmt.Sprint(endpoint + "?" + params.Encode())
	
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}

	wsc.logger.Debugf("Making request to %s", fullURL)
	req = req.WithContext(wsc.ctx)

	resp, err := wsc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	// Log the response for debugging
	wsc.logger.Debugf("Response body: %s", string(body))
	
	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response from server (status %d): %v", resp.StatusCode, string(body))
	}
	
	// For now, accept any 200 OK response as success
	// Different weather services have different success response formats
	// Weather Underground returns "success\n", PWS Weather returns HTML sometimes
	if len(body) == 0 {
		return fmt.Errorf("empty response from server")
	}

	return nil
}

// ValidateReading checks if a reading is valid for submission to weather services
func ValidateReading(r *database.FetchedBucketReading) error {
	if r.Barometer == 0 && r.OutTemp == 0 {
		return fmt.Errorf("rejecting likely faulty reading (temp %v, barometer %v)", r.OutTemp, r.Barometer)
	}
	return nil
}
