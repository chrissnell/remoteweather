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

	"go.uber.org/zap"
)

// PWSWeatherController holds our connection along with some mutexes for operation
type PWSWeatherController struct {
	ctx              context.Context
	wg               *sync.WaitGroup
	config           *Config
	controllerConfig *ControllerConfig
	logger           *zap.SugaredLogger
	fetcher          *TimescaleDBFetcher
}

// PWSWeatherconfig holds configuration for this controller
type PWSWeatherConfig struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	UploadInterval int16  `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

func NewPWSWeatherController(ctx context.Context, wg *sync.WaitGroup, c *Config, controllerConfig *ControllerConfig, logger *zap.SugaredLogger) (*PWSWeatherController, error) {
	pwsc := PWSWeatherController{
		ctx:              ctx,
		wg:               wg,
		config:           c,
		controllerConfig: controllerConfig,
		logger:           logger,
	}

	if pwsc.controllerConfig.PWSWeather.StationID == "" {
		return &PWSWeatherController{}, fmt.Errorf("station ID must be set")
	}

	if pwsc.controllerConfig.PWSWeather.APIKey == "" {
		return &PWSWeatherController{}, fmt.Errorf("API key must be set")
	}

	if pwsc.controllerConfig.PWSWeather.PullFromDevice == "" {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device must be set")
	}

	if pwsc.controllerConfig.PWSWeather.UploadInterval == 0 {
		// Use a default interval of 60 seconds
		pwsc.controllerConfig.PWSWeather.UploadInterval = 60
	}

	pwsc.fetcher = NewTimescaleDBFetcher(c, logger)

	if !pwsc.fetcher.validatePullFromStation(pwsc.controllerConfig.PWSWeather.PullFromDevice) {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device %v is not a valid station name", pwsc.controllerConfig.PWSWeather.PullFromDevice)
	}

	err := pwsc.fetcher.connectToTimescaleDB(c.Storage)
	if err != nil {
		return &PWSWeatherController{}, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

	return &pwsc, nil
}

func (p *PWSWeatherController) StartController() error {
	go p.sendPeriodicReports()
	return nil
}

func (p *PWSWeatherController) sendPeriodicReports() {
	interval, _ := time.ParseDuration(fmt.Sprintf("%vs", p.controllerConfig.PWSWeather.UploadInterval))

	p.wg.Add(1)
	defer p.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug("Sending reading to PWS Weather...")
			br, err := p.fetcher.getReadingsFromTimescaleDB(p.controllerConfig.PWSWeather.PullFromDevice)
			if err != nil {
				log.Info("error getting readings from TimescaleDB:", err)
			}
			log.Debugf("readings fetched from TimescaleDB for PWS Weather: %+v", br)
			err = p.sendReadingsToPWSWeather(&br)
			if err != nil {
				log.Errorf("error sending readings to PWS Weather: %v", err)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *PWSWeatherController) sendReadingsToPWSWeather(r *FetchedBucketReading) error {
	v := url.Values{}

	// Add our authentication parameters to our URL
	v.Set("ID", p.controllerConfig.PWSWeather.StationID)
	v.Set("PASSWORD", p.controllerConfig.PWSWeather.APIKey)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// Set some values for our weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("windgustmph", strconv.FormatInt(int64(r.MaxWindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.OutHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("solarradiation", fmt.Sprintf("%0.2f", r.SolarWatts))
	v.Set("softwaretype", fmt.Sprintf("gopherwx %v", version))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", fmt.Sprint("https://pwsupdate.pwsweather.com/api/v1/submitwx?"+v.Encode()), nil)
	if err != nil {
		return fmt.Errorf("error creating PWS Weather HTTP request: %v", err)
	}

	log.Infof("Making request to PWS weather: https://pwsupdate.pwsweather.com/api/v1/submitwx?%v", v.Encode())
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