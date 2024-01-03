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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PWSWeatherController holds our connection along with some mutexes for operation
type PWSWeatherController struct {
	ctx           context.Context
	wg            *sync.WaitGroup
	config        ControllerConfig
	storageconfig StorageConfig
	deviceconfig  []DeviceConfig
	logger        *zap.SugaredLogger
	db            *gorm.DB
}

// PWSWeatherconfig holds configuration for this controller
type PWSWeatherConfig struct {
	StationID      string `yaml:"station-id,omitempty"`
	APIKey         string `yaml:"api-key,omitempty"`
	UploadInterval int16  `yaml:"upload-interval,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

type FetchedBucketReading struct {
	Bucket                *time.Time `gorm:"column:bucket"`
	StationName           string     `gorm:"column:stationname"`
	Barometer             float32    `gorm:"column:barometer"`
	MaxBarometer          float32    `gorm:"column:max_barometer"`
	MinBarometer          float32    `gorm:"column:min_barometer"`
	InTemp                float32    `gorm:"column:intemp"`
	MaxInTemp             float32    `gorm:"column:max_intemp"`
	MinInTemp             float32    `gorm:"column:max_intemp"`
	ExtraTemp1            float32    `gorm:"column:extratemp1"`
	MaxExtraTemp1         float32    `gorm:"column:min_extratemp1"`
	MinExtraTemp1         float32    `gorm:"column:max_extratemp1"`
	InHumidity            float32    `gorm:"column:inhumidity"`
	MaxInHumidity         float32    `gorm:"column:max_inhumidity"`
	MinInHumidity         float32    `gorm:"column:min_inhumidity"`
	SolarWatts            float32    `gorm:"column:solarwatts"`
	SolarJoules           float32    `gorm:"column:solarjoules"`
	OutTemp               float32    `gorm:"column:outtemp"`
	MaxOutTemp            float32    `gorm:"column:max_outtemp"`
	MinOutTemp            float32    `gorm:"column:min_outtemp"`
	OutHumidity           float32    `gorm:"column:outhumidity"`
	MinOutHumidity        float32    `gorm:"column:min_outhumidity"`
	MaxOutHumidity        float32    `gorm:"column:max_outhumidity"`
	WindSpeed             float32    `gorm:"column:windspeed"`
	MaxWindSpeed          float32    `gorm:"column:max_windspeed"`
	WindDir               float32    `gorm:"column:winddir"`
	WindChill             float32    `gorm:"column:windchill"`
	MinWindChill          float32    `gorm:"column:min_windchill"`
	HeatIndex             float32    `gorm:"column:heatindex"`
	MaxHeatIndex          float32    `gorm:"column:max_heatindex"`
	PeriodRain            float32    `gorm:"column:period_rain"`
	RainRate              float32    `gorm:"column:rainrate"`
	MaxRainRate           float32    `gorm:"column:max_rainrate"`
	DayRain               float32    `gorm:"column:dayrain"`
	MonthRain             float32    `gorm:"column:monthrain"`
	YearRain              float32    `gorm:"column:yearrain"`
	ConsBatteryVoltage    float32    `gorm:"column:consbatteryvoltage"`
	StationBatteryVoltage float32    `gorm:"column:stationbatteryvoltage"`
}

// We implement the Tabler interface for the Reading struct
func (FetchedBucketReading) TableName() string {
	return "weather_1m"
}

func NewPWSWeatherController(ctx context.Context, wg *sync.WaitGroup, c ControllerConfig, s StorageConfig, d []DeviceConfig, logger *zap.SugaredLogger) (*PWSWeatherController, error) {
	pwsc := PWSWeatherController{
		ctx:           ctx,
		wg:            wg,
		config:        c,
		storageconfig: s,
		deviceconfig:  d,
		logger:        logger,
	}

	if pwsc.config.PWSWeather.StationID == "" {
		return &PWSWeatherController{}, fmt.Errorf("station ID must be set")
	}

	if pwsc.config.PWSWeather.APIKey == "" {
		return &PWSWeatherController{}, fmt.Errorf("API key must be set")
	}

	if pwsc.config.PWSWeather.PullFromDevice == "" {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device must be set")
	}

	if !pwsc.validatePullFromStation() {
		return &PWSWeatherController{}, fmt.Errorf("pull-from-device %v is not a valid station name", pwsc.config.PWSWeather.PullFromDevice)
	}

	if pwsc.config.PWSWeather.UploadInterval == 0 {
		// Use a default interval of 60 seconds
		pwsc.config.PWSWeather.UploadInterval = 60
	}

	err := pwsc.connectToTimescaleDB()
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
	interval, _ := time.ParseDuration(fmt.Sprintf("%vs", p.config.PWSWeather.UploadInterval))

	p.wg.Add(1)
	defer p.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Debug("Sending reading to PWS Weather...")
			br, err := p.getReadingsFromTimescaleDB()
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

func (p *PWSWeatherController) connectToTimescaleDB() error {

	var err error

	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(zapLogger),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: false,       // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Use colors
		},
	)

	config := &gorm.Config{
		Logger: dbLogger,
	}

	log.Info("connecting to TimescaleDB...")
	p.db, err = gorm.Open(postgres.Open(p.storageconfig.TimescaleDB.ConnectionString), config)
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}
	log.Info("TimescaleDB connection successful")

	return nil
}

func (p *PWSWeatherController) getReadingsFromTimescaleDB() (FetchedBucketReading, error) {
	var br FetchedBucketReading

	if err := p.db.Table("weather_1m").Where("stationname=? AND bucket > NOW() - INTERVAL '2 minutes'", p.config.PWSWeather.PullFromDevice).Limit(1).Find(&br).Error; err != nil {
		return FetchedBucketReading{}, fmt.Errorf("error querying database for latest readings: %+v", err)
	}

	return br, nil
}

func (p *PWSWeatherController) sendReadingsToPWSWeather(r *FetchedBucketReading) error {
	v := url.Values{}

	// Add our authentication parameters to our URL
	v.Set("ID", p.config.PWSWeather.StationID)
	v.Set("PASSWORD", p.config.PWSWeather.APIKey)

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

func (p *PWSWeatherController) validatePullFromStation() bool {
	if len(p.deviceconfig) > 0 {
		for _, station := range p.deviceconfig {
			if station.Name == p.config.PWSWeather.PullFromDevice {
				return true
			}
		}
	}
	return false
}
