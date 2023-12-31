package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WeatherStationManager holds our active weather station backends
type WeatherStationManager struct {
	Stations []WeatherStation
}

// WeatherStation is an interface that provides standard methods for various
// weather station backends
type WeatherStation interface {
	StartWeatherStation() error
	StationName() string
}

// Reading is a generic weather reading, containing human-readable values
// for most commonly-reported weather metrics.  When creating a new WeatherStation
// implementation, you should ideally use one of the existing Reading struct members.
// If you can't find what you need in here, you can add a new member to the struct.
type Reading struct {
	Timestamp             time.Time `gorm:"column:time"`
	StationName           string    `gorm:"column:stationname"`
	Barometer             float32   `gorm:"column:barometer"`
	InTemp                float32   `gorm:"column:intemp"`
	InHumidity            float32   `gorm:"column:inhumidity"`
	OutTemp               float32   `gorm:"column:outtemp"`
	WindSpeed             float32   `gorm:"column:windspeed"`
	WindSpeed10           float32   `gorm:"column:windspeed10"`
	WindDir               float32   `gorm:"column:winddir"`
	WindChill             float32   `gorm:"column:windchill"`
	HeatIndex             float32   `gorm:"column:heatindex"`
	ExtraTemp1            float32   `gorm:"column:extratemp1"`
	ExtraTemp2            float32   `gorm:"column:extratemp2"`
	ExtraTemp3            float32   `gorm:"column:extratemp3"`
	ExtraTemp4            float32   `gorm:"column:extratemp4"`
	ExtraTemp5            float32   `gorm:"column:extratemp5"`
	ExtraTemp6            float32   `gorm:"column:extratemp6"`
	ExtraTemp7            float32   `gorm:"column:extratemp7"`
	SoilTemp1             float32   `gorm:"column:soiltemp1"`
	SoilTemp2             float32   `gorm:"column:soiltemp2"`
	SoilTemp3             float32   `gorm:"column:soiltemp3"`
	SoilTemp4             float32   `gorm:"column:soiltemp4"`
	LeafTemp1             float32   `gorm:"column:leaftemp1"`
	LeafTemp2             float32   `gorm:"column:leaftemp2"`
	LeafTemp3             float32   `gorm:"column:leaftemp3"`
	LeafTemp4             float32   `gorm:"column:leaftemp4"`
	OutHumidity           float32   `gorm:"column:outhumidity"`
	ExtraHumidity1        float32   `gorm:"column:extrahumidity1"`
	ExtraHumidity2        float32   `gorm:"column:extrahumidity2"`
	ExtraHumidity3        float32   `gorm:"column:extrahumidity3"`
	ExtraHumidity4        float32   `gorm:"column:extrahumidity4"`
	ExtraHumidity5        float32   `gorm:"column:extrahumidity5"`
	ExtraHumidity6        float32   `gorm:"column:extrahumidity6"`
	ExtraHumidity7        float32   `gorm:"column:extrahumidity7"`
	RainRate              float32   `gorm:"column:rainrate"`
	RainIncremental       float32   `gorm:"column:rainincremental"`
	SolarWatts            float32   `gorm:"column:solarwatts"`
	SolarJoules           float32   `gorm:"column:solarjoules"`
	UV                    float32   `gorm:"column:uv"`
	Radiation             float32   `gorm:"column:radiation"`
	StormRain             float32   `gorm:"column:stormrain"`
	StormStart            time.Time `gorm:"column:stormstart"`
	DayRain               float32   `gorm:"column:dayrain"`
	MonthRain             float32   `gorm:"column:monthrain"`
	YearRain              float32   `gorm:"column:yearrain"`
	DayET                 float32   `gorm:"column:dayet"`
	MonthET               float32   `gorm:"column:monthet"`
	YearET                float32   `gorm:"column:yearet"`
	SoilMoisture1         float32   `gorm:"column:soilmoisture1"`
	SoilMoisture2         float32   `gorm:"column:soilmoisture2"`
	SoilMoisture3         float32   `gorm:"column:soilmoisture3"`
	SoilMoisture4         float32   `gorm:"column:soilmoisture4"`
	LeafWetness1          float32   `gorm:"column:leafwetness1"`
	LeafWetness2          float32   `gorm:"column:leafwetness2"`
	LeafWetness3          float32   `gorm:"column:leafwetness3"`
	LeafWetness4          float32   `gorm:"column:leafwetness4"`
	InsideAlarm           uint8     `gorm:"column:insidealarm"`
	RainAlarm             uint8     `gorm:"column:rainalarm"`
	OutsideAlarm1         uint8     `gorm:"column:outsidealarm1"`
	OutsideAlarm2         uint8     `gorm:"column:outsidealarm2"`
	ExtraAlarm1           uint8     `gorm:"column:extraalarm1"`
	ExtraAlarm2           uint8     `gorm:"column:extraalarm2"`
	ExtraAlarm3           uint8     `gorm:"column:extraalarm3"`
	ExtraAlarm4           uint8     `gorm:"column:extraalarm4"`
	ExtraAlarm5           uint8     `gorm:"column:extraalarm5"`
	ExtraAlarm6           uint8     `gorm:"column:extraalarm6"`
	ExtraAlarm7           uint8     `gorm:"column:extraalarm7"`
	ExtraAlarm8           uint8     `gorm:"column:extraalarm8"`
	SoilLeafAlarm1        uint8     `gorm:"column:soilleafalarm1"`
	SoilLeafAlarm2        uint8     `gorm:"column:soilleafalarm2"`
	SoilLeafAlarm3        uint8     `gorm:"column:soilleafalarm3"`
	SoilLeafAlarm4        uint8     `gorm:"column:soilleafalarm4"`
	TxBatteryStatus       uint8     `gorm:"column:txbatterystatus"`
	ConsBatteryVoltage    float32   `gorm:"column:consbatteryvoltage"`
	StationBatteryVoltage float32   `gorm:"column:stationbatteryvoltage"`
	ForecastIcon          uint8     `gorm:"column:forecasticon"`
	ForecastRule          uint8     `gorm:"column:forecastrule"`
	Sunrise               time.Time `gorm:"column:sunrise"`
	Sunset                time.Time `gorm:"column:sunset"`
}

// NewWeatherStationManager creats a WeatherStationManager object, populated with all configured
// WeatherStationEngines
func NewWeatherStationManager(ctx context.Context, wg *sync.WaitGroup, c *Config, distributor chan Reading, logger *zap.SugaredLogger) (*WeatherStationManager, error) {
	wsm := WeatherStationManager{}

	for _, s := range c.Devices {
		switch s.Type {
		case "davis":
			log.Infof("Initializing Davis weather station [%v]", s.Name)
			// Create a new DavisWeatherStation and pass the config for this station
			station, err := NewDavisWeatherStation(ctx, wg, s, distributor, logger)
			if err != nil {
				return &wsm, fmt.Errorf("error creating Davis weather station: %v", err)
			}
			wsm.Stations = append(wsm.Stations, station)
		case "campbellscientific":
			log.Infof("Initializing Campbell Scientific weather station [%v]", s.Name)
			// Create a new CampbellScientificWeatherStation and pass the config for this station
			station, err := NewCampbellScientificWeatherStation(ctx, wg, s, distributor, logger)
			if err != nil {
				return &wsm, fmt.Errorf("error creating Campbell Scientific weather station: %v", err)
			}
			wsm.Stations = append(wsm.Stations, station)
		}
	}

	return &wsm, nil
}

func (wsm *WeatherStationManager) StartWeatherStations() error {
	var err error

	for _, station := range wsm.Stations {
		log.Infof("Starting weather station %v ...", station.StationName())
		err = station.StartWeatherStation()
		if err != nil {
			return err
		}
	}

	return nil
}

func calcWindChill(temp float32, windspeed float32) float32 {
	// For wind speeds < 3 or temps > 50, wind chill is just the current temperature
	if (temp > 50) || (windspeed < 3) {
		return temp
	}

	w64 := float64(windspeed)
	return (35.74 + (0.6215 * temp) - (35.75 * float32(math.Pow(w64, 0.16))) + (0.4275 * temp * float32(math.Pow(w64, 0.16))))
}

func calcHeatIndex(temp float32, humidity float32) float32 {

	// Heat indices don't make much sense at temps below 77° F, so just return the current temperature
	if temp < 77 {
		return temp
	}

	// First, we try Steadman's method, which is valid for all heat indices
	// below 80° F
	hi := 0.5 * (temp + 61.0 + ((temp - 68.0) * 1.2) + (humidity + 0.094))
	if hi < 80 {
		// Only return heat index if it's greater than the temperature
		if hi > temp {
			return hi
		}
		return temp
	}

	// Our heat index is > 80, so we need to use the Rothfusz method instead
	c1 := -42.379
	c2 := 2.04901523
	c3 := 10.14333127
	c4 := 0.22475541
	c5 := 0.00683783
	c6 := 0.05481717
	c7 := 0.00122874
	c8 := 0.00085282
	c9 := 0.00000199

	t64 := float64(temp)
	h64 := float64(humidity)

	hi64 := c1 + (c2 * t64) + (c3 * h64) - (c4 * t64 * h64) - (c5 * math.Pow(t64, 2)) - (c6 * math.Pow(h64, 2)) + (c7 * math.Pow(t64, 2) * h64) + (c8 * t64 * math.Pow(h64, 2)) - (c9 * math.Pow(t64, 2) * math.Pow(h64, 2))

	// If RH < 13% and temperature is between 80 and 112, we need to subtract an adjustment
	if humidity < 13 && temp >= 80 && temp <= 112 {
		adj := ((13 - h64) / 4) * math.Sqrt((17-math.Abs(t64-95.0))/17)
		hi64 = hi64 - adj
	} else if humidity > 80 && temp >= 80 && temp <= 87 {
		// Likewise, if RH > 80% and temperature is between 80 and 87, we need to add an adjustment
		adj := ((h64 - 85.0) / 10) * ((87.0 - t64) / 5)
		hi64 = hi64 + adj
	}

	// Only return heat index if it's greater than the temperature
	if hi64 > t64 {
		return float32(hi64)
	}
	return temp
}
