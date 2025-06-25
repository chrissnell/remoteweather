package main

import (
	"fmt"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type TimescaleDBClient struct {
	config *Config
	logger *zap.SugaredLogger
	db     *gorm.DB
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
	PotentialSolarWatts   float32    `gorm:"column:potentialsolarwatts"`
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

func NewTimescaleDBClient(c *Config, logger *zap.SugaredLogger) *TimescaleDBClient {
	return &TimescaleDBClient{
		config: c,
		logger: logger,
	}
}

// We implement the Tabler interface for the Reading struct
func (FetchedBucketReading) TableName() string {
	return "weather_1m"
}

func (t *TimescaleDBClient) connectToTimescaleDB() error {

	var err error

	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(log.GetZapLogger()),
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
	t.db, err = gorm.Open(postgres.Open(t.config.Storage.TimescaleDB.ConnectionString), config)
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}
	log.Info("TimescaleDB connection successful")

	return nil
}

func (p *TimescaleDBClient) getReadingsFromTimescaleDB(pullFromDevice string) (FetchedBucketReading, error) {
	var br FetchedBucketReading

	if err := p.db.Table("weather_1m").Where("stationname=? AND bucket > NOW() - INTERVAL '2 minutes'", pullFromDevice).Limit(1).Find(&br).Error; err != nil {
		return FetchedBucketReading{}, fmt.Errorf("error querying database for latest readings: %+v", err)
	}

	return br, nil
}

func (t *TimescaleDBClient) validatePullFromStation(pullFromDevice string) bool {
	if len(t.config.Devices) > 0 {
		for _, station := range t.config.Devices {
			if station.Name == pullFromDevice {
				return true
			}
		}
	}
	return false
}
