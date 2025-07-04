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
	WeatherUndergroundConfig config.WeatherUndergroundData
}

func NewWeatherUndergroundController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, wuconfig config.WeatherUndergroundData, logger *zap.SugaredLogger) (*WeatherUndergroundController, error) {
	// Create base weather service controller
	base, err := controllers.NewWeatherServiceController(ctx, wg, configProvider, logger)
	if err != nil {
		return nil, err
	}

	// Validate Weather Underground specific configuration
	serviceConfig := controllers.WeatherServiceConfig{
		ServiceName:    "Weather Underground",
		StationID:      wuconfig.StationID,
		APIKey:         wuconfig.APIKey,
		PullFromDevice: wuconfig.PullFromDevice,
	}

	if err := controllers.ValidateWeatherServiceConfig(serviceConfig); err != nil {
		return nil, err
	}

	// Set defaults
	if wuconfig.APIEndpoint == "" {
		wuconfig.APIEndpoint = "https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php"
	}
	if wuconfig.UploadInterval == "" {
		wuconfig.UploadInterval = "60"
	}

	// Validate pull-from-device exists
	if !base.DB.ValidatePullFromStation(wuconfig.PullFromDevice) {
		return nil, fmt.Errorf("pull-from-device %v is not a valid station name", wuconfig.PullFromDevice)
	}

	return &WeatherUndergroundController{
		WeatherServiceController: base,
		WeatherUndergroundConfig: wuconfig,
	}, nil
}

func (p *WeatherUndergroundController) StartController() error {
	log.Info("Starting Weather Underground controller...")
	go p.sendPeriodicReports()
	return nil
}

func (p *WeatherUndergroundController) sendPeriodicReports() {
	config := controllers.WeatherServiceConfig{
		ServiceName:    "Weather Underground",
		StationID:      p.WeatherUndergroundConfig.StationID,
		APIKey:         p.WeatherUndergroundConfig.APIKey,
		APIEndpoint:    p.WeatherUndergroundConfig.APIEndpoint,
		UploadInterval: p.WeatherUndergroundConfig.UploadInterval,
		PullFromDevice: p.WeatherUndergroundConfig.PullFromDevice,
	}

	p.StartPeriodicReports(config, p.sendReadingsToWeatherUnderground)
}

func (p *WeatherUndergroundController) sendReadingsToWeatherUnderground(r *database.FetchedBucketReading) error {
	if err := controllers.ValidateReading(r); err != nil {
		return err
	}

	v := url.Values{}

	// Add authentication parameters
	v.Set("ID", p.WeatherUndergroundConfig.StationID)
	v.Set("PASSWORD", p.WeatherUndergroundConfig.APIKey)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// This is a real-time weather update request (approx 2.5s interval)
	v.Set("action", "updateraw")
	v.Set("realtime", "1")
	v.Set("rtfreq", "2.5")

	// Set weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.InHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("softwaretype", fmt.Sprintf("RemoteWeather %v", constants.Version))

	return p.SendHTTPRequest(p.WeatherUndergroundConfig.APIEndpoint, v)
}
