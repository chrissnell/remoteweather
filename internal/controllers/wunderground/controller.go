package wunderground

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"go.uber.org/zap"
)

// WeatherUndergroundController holds our connection along with some mutexes for operation
type WeatherUndergroundController struct {
	ctx                      context.Context
	wg                       *sync.WaitGroup
	config                   *types.Config
	WeatherUndergroundConfig types.WeatherUndergroundConfig
	logger                   *zap.SugaredLogger
	DB                       *database.Client
}

func NewWeatherUndergroundController(ctx context.Context, wg *sync.WaitGroup, c *types.Config, wuconfig types.WeatherUndergroundConfig, logger *zap.SugaredLogger) (*WeatherUndergroundController, error) {
	wuc := WeatherUndergroundController{
		ctx:                      ctx,
		wg:                       wg,
		config:                   c,
		WeatherUndergroundConfig: wuconfig,
		logger:                   logger,
	}

	if wuc.config.Storage.TimescaleDB.ConnectionString == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("TimescaleDB storage must be configured for the Weather Underground controller to function")
	}

	if wuc.WeatherUndergroundConfig.StationID == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("station ID must be set")
	}

	if wuc.WeatherUndergroundConfig.APIKey == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("API key must be set")
	}

	if wuc.WeatherUndergroundConfig.PullFromDevice == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("pull-from-device must be set")
	}

	if wuc.WeatherUndergroundConfig.APIEndpoint == "" {
		wuc.WeatherUndergroundConfig.APIEndpoint = "https://weatherstation.wunderground.com/weatherstation/updateweatherstation.php"
	}

	if wuc.WeatherUndergroundConfig.UploadInterval == "" {
		// Use a default interval of 60 seconds
		wuc.WeatherUndergroundConfig.UploadInterval = "60"
	}

	wuc.DB = database.NewClient(c, logger)

	if !wuc.DB.ValidatePullFromStation(wuc.WeatherUndergroundConfig.PullFromDevice) {
		return &WeatherUndergroundController{}, fmt.Errorf("pull-from-device %v is not a valid station name", wuc.WeatherUndergroundConfig.PullFromDevice)
	}

	err := wuc.DB.ConnectToTimescaleDB()
	if err != nil {
		return &WeatherUndergroundController{}, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

	return &wuc, nil
}

func (p *WeatherUndergroundController) StartController() error {
	log.Info("Starting Weather Underground controller...")
	go p.sendPeriodicReports()
	return nil
}

func (p *WeatherUndergroundController) sendPeriodicReports() {
	p.wg.Add(1)
	defer p.wg.Done()

	submitInterval, err := time.ParseDuration(fmt.Sprintf("%vs", p.WeatherUndergroundConfig.UploadInterval))
	if err != nil {
		log.Errorf("error parsing duration: %v", err)
	}

	ticker := time.NewTicker(submitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug("Sending reading to Weather Underground...")
			br, err := p.DB.GetReadingsFromTimescaleDB(p.WeatherUndergroundConfig.PullFromDevice)
			if err != nil {
				log.Info("error getting readings from TimescaleDB:", err)
			}
			log.Debugf("readings fetched from TimescaleDB for Weather Underground: %+v", br)
			err = p.sendReadingsToWeatherUnderground(&br)
			if err != nil {
				log.Errorf("error sending readings to Weather Underground: %v", err)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *WeatherUndergroundController) sendReadingsToWeatherUnderground(r *database.FetchedBucketReading) error {
	v := url.Values{}

	// Add our authentication parameters to our URL
	v.Set("ID", p.WeatherUndergroundConfig.StationID)
	v.Set("PASSWORD", p.WeatherUndergroundConfig.APIKey)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// This is a real-time weather update request (approx 2.5s interval)
	v.Set("action", "updateraw")
	v.Set("realtime", "1")
	v.Set("rtfreq", "2.5")

	// Set some values for our weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.InHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("softwaretype", fmt.Sprintf("RemoteWeather %v", constants.Version))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", fmt.Sprint(p.WeatherUndergroundConfig.APIEndpoint+"?"+v.Encode()), nil)
	if err != nil {
		return fmt.Errorf("error creating Weather Underground HTTP request: %v", err)
	}

	log.Debugf("Making request to Weather Underground: %v?%v", p.WeatherUndergroundConfig.APIEndpoint, v.Encode())
	req = req.WithContext(p.ctx)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending report to Weather Underground: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("error reading Weather Underground response body: %v", err)
	}

	if !bytes.Contains(body, []byte("success")) {
		return fmt.Errorf("bad response from Weather Underground server: %v", string(body))
	}

	return nil
}
