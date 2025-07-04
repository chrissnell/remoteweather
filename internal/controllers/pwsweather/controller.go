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

	v := url.Values{}

	// Add authentication parameters
	v.Set("ID", p.PWSWeatherConfig.StationID)
	v.Set("PASSWORD", p.PWSWeatherConfig.APIKey)

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

func (p *PWSWeatherController) fetchReadingsFromTimescaleDB() (database.FetchedBucketReading, error) {
	// Implementation of fetchReadingsFromTimescaleDB method
	// This method is not provided in the original file or the new file
	// It's assumed to exist as it's called in the sendPeriodicReports method
	return database.FetchedBucketReading{}, nil // Placeholder return, actual implementation needed
}
