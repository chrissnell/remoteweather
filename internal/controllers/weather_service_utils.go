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

	// Perform initial upload after 15 seconds
	wsc.logger.Infof("%s controller will perform initial upload in 15 seconds...", config.ServiceName)
	initialTimer := time.NewTimer(15 * time.Second)
	select {
	case <-initialTimer.C:
		wsc.logger.Debugf("Performing initial %s upload...", config.ServiceName)
		br, err := wsc.DB.GetReadingsFromTimescaleDB(config.PullFromDevice)
		if err != nil {
			wsc.logger.Errorf("error getting initial readings from TimescaleDB: %v", err)
		} else {
			if err := sendFunc(&br); err != nil {
				wsc.logger.Errorf("error sending initial readings to %s: %v", config.ServiceName, err)
			} else {
				wsc.logger.Infof("Initial %s upload successful", config.ServiceName)
			}
		}
	case <-wsc.ctx.Done():
		initialTimer.Stop()
		return
	}

	// Continue with regular periodic reports
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

// GetDevices returns all configured devices
func (wsc *WeatherServiceController) GetDevices() ([]config.DeviceData, error) {
	return wsc.configProvider.GetDevices()
}

// SendHTTPRequest sends an HTTP GET request with URL-encoded parameters
func (wsc *WeatherServiceController) SendHTTPRequest(endpoint string, params url.Values) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", endpoint, params.Encode()), nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}

	wsc.logger.Debugf("Making request to %s?%s", endpoint, params.Encode())
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
	if resp.StatusCode >= 400 {
		return fmt.Errorf("bad response from server (status %d): %v", resp.StatusCode, string(body))
	}
	
	// For successful status codes (200-399), we accept the response
	// Different weather services have different success response formats

	return nil
}

// ValidateReading checks if a reading is valid for submission to weather services
func ValidateReading(r *database.FetchedBucketReading) error {
	if r.Barometer == 0 && r.OutTemp == 0 {
		return fmt.Errorf("rejecting likely faulty reading (temp %v, barometer %v)", r.OutTemp, r.Barometer)
	}
	return nil
}
