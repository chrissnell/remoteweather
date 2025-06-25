package main

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
	"github.com/chrissnell/remoteweather/internal/log"
	"go.uber.org/zap"
)

// WeatherUndergroundController holds our connection along with some mutexes for operation
type WeatherUndergroundController struct {
	ctx      context.Context
	wg       *sync.WaitGroup
	config   *Config
	wuconfig WeatherUndergroundConfig
	logger   *zap.SugaredLogger
	DB       *TimescaleDBClient
}

func NewWeatherUndergroundController(ctx context.Context, wg *sync.WaitGroup, c *Config, wuconfig WeatherUndergroundConfig, logger *zap.SugaredLogger) (*WeatherUndergroundController, error) {
	wuc := WeatherUndergroundController{
		ctx:      ctx,
		wg:       wg,
		config:   c,
		wuconfig: wuconfig,
		logger:   logger,
	}

	if wuc.config.Storage.TimescaleDB.ConnectionString == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("TimescaleDB storage must be configured for the Weather Underground controller to function")
	}

	if wuc.wuconfig.StationID == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("station ID must be set")
	}

	if wuc.wuconfig.APIKey == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("API key must be set")
	}

	if wuc.wuconfig.PullFromDevice == "" {
		return &WeatherUndergroundController{}, fmt.Errorf("pull-from-device must be set")
	}

	if wuc.wuconfig.APIEndpoint == "" {
		wuc.wuconfig.APIEndpoint = "https://rtupdate.wunderground.com/weatherstation/updateweatherstation.php"
	}

	if wuc.wuconfig.UploadInterval == "" {
		// Use a default interval of 60 seconds
		wuc.wuconfig.UploadInterval = "60"
	}

	wuc.DB = NewTimescaleDBClient(c, logger)

	if !wuc.DB.ValidatePullFromStation(wuc.wuconfig.PullFromDevice) {
		return &WeatherUndergroundController{}, fmt.Errorf("pull-from-device %v is not a valid station name", wuc.wuconfig.PullFromDevice)
	}

	err := wuc.DB.ConnectToTimescaleDB()
	if err != nil {
		return &WeatherUndergroundController{}, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

	return &wuc, nil
}

func (p *WeatherUndergroundController) StartController() error {
	go p.sendPeriodicReports()
	return nil
}

func (p *WeatherUndergroundController) sendPeriodicReports() {
	p.wg.Add(1)
	defer p.wg.Done()

	submitInterval, err := time.ParseDuration(fmt.Sprintf("%vs", p.wuconfig.UploadInterval))
	if err != nil {
		log.Errorf("error parsing duration: %v", err)
	}

	ticker := time.NewTicker(submitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug("Sending reading to PWS Weather...")
			br, err := p.DB.GetReadingsFromTimescaleDB(p.wuconfig.PullFromDevice)
			if err != nil {
				log.Info("error getting readings from TimescaleDB:", err)
			}
			log.Debugf("readings fetched from TimescaleDB for PWS Weather: %+v", br)
			err = p.sendReadingsToWeatherUnderground(&br)
			if err != nil {
				log.Errorf("error sending readings to PWS Weather: %v", err)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *WeatherUndergroundController) sendReadingsToWeatherUnderground(r *FetchedBucketReading) error {
	v := url.Values{}

	// Add our authentication parameters to our URL
	v.Set("ID", p.wuconfig.StationID)
	v.Set("PASSWORD", p.wuconfig.APIKey)

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

	req, err := http.NewRequest("GET", fmt.Sprint(p.wuconfig.APIEndpoint+"?"+v.Encode()), nil)
	if err != nil {
		return fmt.Errorf("error creating PWS Weather HTTP request: %v", err)
	}

	log.Debugf("Making request to Weather Underground: %v?%v", p.wuconfig.APIEndpoint, v.Encode())
	req = req.WithContext(p.ctx)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending report to PWS Weather: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("error reading PWS Weather response body: %v", err)
	}

	if !bytes.Contains(body, []byte("success")) {
		return fmt.Errorf("bad response from PWS Weather server: %v", string(body))
	}

	return nil
}
