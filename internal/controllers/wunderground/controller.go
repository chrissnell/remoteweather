// Package wunderground provides integration with Weather Underground service for uploading weather data.
package wunderground

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// WeatherUndergroundController holds our connection along with some mutexes for operation
type WeatherUndergroundController struct {
	*controllers.WeatherServiceController
}

func NewWeatherUndergroundController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, logger *zap.SugaredLogger) (*WeatherUndergroundController, error) {
	// Create base weather service controller
	base, err := controllers.NewWeatherServiceController(ctx, wg, configProvider, logger)
	if err != nil {
		return nil, err
	}

	return &WeatherUndergroundController{
		WeatherServiceController: base,
	}, nil
}

func (p *WeatherUndergroundController) StartController() error {
	log.Info("Starting Weather Underground controller...")
	go p.sendPeriodicReports()
	return nil
}

func (p *WeatherUndergroundController) sendPeriodicReports() {
	// Get all devices with Weather Underground enabled
	devices, err := p.WeatherServiceController.GetDevices()
	if err != nil {
		log.Errorf("Error getting devices: %v", err)
		return
	}

	// Count WU-enabled devices
	enabledCount := 0
	for _, device := range devices {
		if device.WUEnabled && device.WUStationID != "" && device.WUPassword != "" {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		log.Info("No Weather Underground enabled devices found")
		return
	}

	log.Infof("Found %d Weather Underground enabled device(s)", enabledCount)

	// Start monitoring for each WU-enabled device
	for _, device := range devices {
		if device.WUEnabled && device.WUStationID != "" && device.WUPassword != "" {
			log.Infof("Starting Weather Underground monitoring for device: %s (Station ID: %s)", device.Name, device.WUStationID)
			
			// Use device-specific upload interval or default
			uploadInterval := "60"
			if device.WUUploadInterval > 0 {
				uploadInterval = strconv.Itoa(device.WUUploadInterval)
			}

			// Create a copy of device for closure
			deviceCopy := device

			// Use device's API endpoint or default
			apiEndpoint := device.WUAPIEndpoint
			if apiEndpoint == "" {
				apiEndpoint = "https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php"
			}

			config := controllers.WeatherServiceConfig{
				ServiceName:    "Weather Underground",
				StationID:      device.WUStationID,
				APIKey:         device.WUPassword,
				APIEndpoint:    apiEndpoint,
				UploadInterval: uploadInterval,
				PullFromDevice: device.Name,
			}

			// Start monitoring in separate goroutine for each device
			go p.StartPeriodicReports(config, func(r *database.FetchedBucketReading) error {
				return p.sendReadingsToWeatherUnderground(deviceCopy, r)
			})
		}
	}
}

func (p *WeatherUndergroundController) sendReadingsToWeatherUnderground(device config.DeviceData, r *database.FetchedBucketReading) error {
	if err := controllers.ValidateReading(r); err != nil {
		return err
	}

	v := url.Values{}

	// Add authentication parameters from device
	v.Set("ID", device.WUStationID)
	v.Set("PASSWORD", device.WUPassword)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// This is a real-time weather update request (approx 2.5s interval)
	v.Set("action", "updateraw")
	v.Set("realtime", "1")
	v.Set("rtfreq", "2.5")

	// Set weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.OutHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("softwaretype", fmt.Sprintf("RemoteWeather %v", constants.Version))

	// Use device's API endpoint or default
	apiEndpoint := device.WUAPIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php"
	}

	return p.SendHTTPRequest(apiEndpoint, v)
}
