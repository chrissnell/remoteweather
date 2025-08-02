// Package pwsweather provides integration with PWS Weather service for uploading weather data.
package pwsweather

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

	// Set defaults for global settings
	if pwc.APIEndpoint == "" {
		pwc.APIEndpoint = "https://pwsupdate.pwsweather.com/api/v1/submitwx"
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
	// Get all devices with PWS enabled
	devices, err := p.WeatherServiceController.GetDevices()
	if err != nil {
		log.Errorf("Error getting devices: %v", err)
		return
	}

	// Count PWS-enabled devices
	enabledCount := 0
	for _, device := range devices {
		if device.PWSEnabled && device.PWSStationID != "" && device.PWSPassword != "" {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		log.Info("No PWS Weather enabled devices found")
		return
	}

	log.Infof("Found %d PWS Weather enabled device(s)", enabledCount)

	// Start monitoring for each PWS-enabled device
	for _, device := range devices {
		if device.PWSEnabled && device.PWSStationID != "" && device.PWSPassword != "" {
			log.Infof("Starting PWS Weather monitoring for device: %s (Station ID: %s)", device.Name, device.PWSStationID)
			
			// Use device-specific upload interval or default
			uploadInterval := "60"
			if device.PWSUploadInterval > 0 {
				uploadInterval = strconv.Itoa(device.PWSUploadInterval)
			}

			// Create a copy of device for closure
			deviceCopy := device

			config := controllers.WeatherServiceConfig{
				ServiceName:    "PWS Weather",
				StationID:      device.PWSStationID,
				APIKey:         device.PWSPassword,
				APIEndpoint:    p.PWSWeatherConfig.APIEndpoint,
				UploadInterval: uploadInterval,
				PullFromDevice: device.Name,
			}

			// Start monitoring in separate goroutine for each device
			go p.StartPeriodicReports(config, func(r *database.FetchedBucketReading) error {
				return p.sendReadingsToPWSWeather(deviceCopy, r)
			})
		}
	}
}

func (p *PWSWeatherController) sendReadingsToPWSWeather(device config.DeviceData, r *database.FetchedBucketReading) error {
	if err := controllers.ValidateReading(r); err != nil {
		return err
	}

	v := url.Values{}

	// Add authentication parameters from device
	v.Set("ID", device.PWSStationID)
	v.Set("PASSWORD", device.PWSPassword)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// Set weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("windgustmph", strconv.FormatInt(int64(r.MaxWindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.OutHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("solarradiation", fmt.Sprintf("%0.2f", r.SolarWatts))
	v.Set("softwaretype", fmt.Sprintf("RemoteWeather-%v", constants.Version))

	return p.SendHTTPRequest(p.PWSWeatherConfig.APIEndpoint, v)
}