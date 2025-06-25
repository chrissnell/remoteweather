package pwsweather

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

// PWSWeatherController holds our connection along with some mutexes for operation
type PWSWeatherController struct {
	ctx              context.Context
	wg               *sync.WaitGroup
	config           *types.Config
	PWSWeatherConfig types.PWSWeatherConfig
	logger           *zap.SugaredLogger
	DB               *database.Client
}

func NewPWSWeatherController(ctx context.Context, wg *sync.WaitGroup, c *types.Config, pwc types.PWSWeatherConfig, logger *zap.SugaredLogger) (*PWSWeatherController, error) {
	pwsc := PWSWeatherController{
		ctx:              ctx,
		wg:               wg,
		config:           c,
		PWSWeatherConfig: pwc,
		logger:           logger,
	}

	if pwsc.config.Storage.TimescaleDB.ConnectionString == "" {
		return &PWSWeatherController{}, fmt.Errorf("TimescaleDB storage must be configured for the PWS Weather controller to function")
	}

	if pwsc.PWSWeatherConfig.StationID == "" {
		return &PWSWeatherController{}, fmt.Errorf("station ID must be set")
	}

	if pwsc.PWSWeatherConfig.APIKey == "" {
		return &PWSWeatherController{}, fmt.Errorf("API key must be set")
	}

	if pwsc.PWSWeatherConfig.PullFromDevice == "" {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device must be set")
	}

	if pwsc.PWSWeatherConfig.APIEndpoint == "" {
		pwsc.PWSWeatherConfig.APIEndpoint = "https://pwsupdate.pwsweather.com/api/v1/submitwx"
	}

	if pwsc.PWSWeatherConfig.UploadInterval == "" {
		// Use a default interval of 60 seconds
		pwsc.PWSWeatherConfig.UploadInterval = "60"
	}

	pwsc.DB = database.NewClient(c, logger)

	if !pwsc.DB.ValidatePullFromStation(pwsc.PWSWeatherConfig.PullFromDevice) {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device %v is not a valid station name", pwsc.PWSWeatherConfig.PullFromDevice)
	}

	err := pwsc.DB.ConnectToTimescaleDB()
	if err != nil {
		return &PWSWeatherController{}, fmt.Errorf("could not connect to TimescaleDB: %v", err)
	}

	return &pwsc, nil
}

func (p *PWSWeatherController) StartController() error {
	log.Info("Starting PWS Weather controller...")
	go p.sendPeriodicReports()
	return nil
}

func (p *PWSWeatherController) sendPeriodicReports() {
	p.wg.Add(1)
	defer p.wg.Done()

	submitInterval, err := time.ParseDuration(fmt.Sprintf("%vs", p.PWSWeatherConfig.UploadInterval))
	if err != nil {
		log.Errorf("error parsing duration: %v", err)
	}

	ticker := time.NewTicker(submitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug("Sending reading to PWS Weather...")
			br, err := p.DB.GetReadingsFromTimescaleDB(p.PWSWeatherConfig.PullFromDevice)
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

func (p *PWSWeatherController) sendReadingsToPWSWeather(r *database.FetchedBucketReading) error {
	v := url.Values{}

	if r.Barometer == 0 && r.OutTemp == 0 {
		return fmt.Errorf("rejecting likely faulty reading (temp %v, barometer %v)", r.OutTemp, r.Barometer)
	}

	// Add our authentication parameters to our URL
	v.Set("ID", p.PWSWeatherConfig.StationID)
	v.Set("PASSWORD", p.PWSWeatherConfig.APIKey)

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
	v.Set("softwaretype", fmt.Sprintf("RemoteWeather-%v", constants.Version))

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", fmt.Sprint(p.PWSWeatherConfig.APIEndpoint+"?"+v.Encode()), nil)
	if err != nil {
		return fmt.Errorf("error creating PWS Weather HTTP request: %v", err)
	}

	log.Debugf("Making request to PWS weather: %v?%v", p.PWSWeatherConfig.APIEndpoint, v.Encode())
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

func (p *PWSWeatherController) fetchReadingsFromTimescaleDB() (database.FetchedBucketReading, error) {
	// Implementation of fetchReadingsFromTimescaleDB method
	// This method is not provided in the original file or the new file
	// It's assumed to exist as it's called in the sendPeriodicReports method
	return database.FetchedBucketReading{}, nil // Placeholder return, actual implementation needed
}
