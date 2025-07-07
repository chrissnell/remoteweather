package restserver

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)

// Time constants
const (
	Day   = 24 * time.Hour
	Month = Day * 30
)

// AerisWeatherForecastRecord represents forecast data from Aeris Weather
type AerisWeatherForecastRecord struct {
	gorm.Model

	ForecastSpanHours int16        `gorm:"uniqueIndex:idx_location_span,not null"`
	Location          string       `gorm:"uniqueIndex:idx_location_span,not null"`
	Data              pgtype.JSONB `gorm:"type:jsonb;default:'[]';not null"`
}

// TableName implements the GORM Tabler interface to specify the correct table name
func (AerisWeatherForecastRecord) TableName() string {
	return "aeris_weather_forecasts"
}

// WeatherReading represents a weather reading for JSON output
type WeatherReading struct {
	StationName      string `json:"stationname"`
	StationType      string `json:"stationtype,omitempty"`
	ReadingTimestamp int64  `json:"ts"`
	// Using pointers for readings ensures that json.Marshall will encode zeros as 0
	// instead of simply not including the field in the data structure
	OutsideTemperature    json.Number `json:"otemp,omitempty"`
	ExtraTemp1            json.Number `json:"extratemp1,omitempty"`
	ExtraTemp2            json.Number `json:"extratemp2,omitempty"`
	ExtraTemp3            json.Number `json:"extratemp3,omitempty"`
	ExtraTemp4            json.Number `json:"extratemp4,omitempty"`
	ExtraTemp5            json.Number `json:"extratemp5,omitempty"`
	ExtraTemp6            json.Number `json:"extratemp6,omitempty"`
	ExtraTemp7            json.Number `json:"extratemp7,omitempty"`
	SoilTemp1             json.Number `json:"soiltemp1,omitempty"`
	SoilTemp2             json.Number `json:"soiltemp2,omitempty"`
	SoilTemp3             json.Number `json:"soiltemp3,omitempty"`
	SoilTemp4             json.Number `json:"soiltemp4,omitempty"`
	LeafTemp1             json.Number `json:"leaftemp1,omitempty"`
	LeafTemp2             json.Number `json:"leaftemp2,omitempty"`
	LeafTemp3             json.Number `json:"leaftemp3,omitempty"`
	LeafTemp4             json.Number `json:"leaftemp4,omitempty"`
	OutHumidity           json.Number `json:"outhumidity,omitempty"`
	ExtraHumidity1        json.Number `json:"extrahumidity1,omitempty"`
	ExtraHumidity2        json.Number `json:"extrahumidity2,omitempty"`
	ExtraHumidity3        json.Number `json:"extrahumidity3,omitempty"`
	ExtraHumidity4        json.Number `json:"extrahumidity4,omitempty"`
	ExtraHumidity5        json.Number `json:"extrahumidity5,omitempty"`
	ExtraHumidity6        json.Number `json:"extrahumidity6,omitempty"`
	ExtraHumidity7        json.Number `json:"extrahumidity7,omitempty"`
	OutsideHumidity       json.Number `json:"ohum,omitempty"`
	RainRate              json.Number `json:"rainrate,omitempty"`
	RainIncremental       json.Number `json:"rainincremental,omitempty"`
	PeriodRain            json.Number `json:"period_rain,omitempty"`
	SolarWatts            json.Number `json:"solarwatts,omitempty"`
	PotentialSolarWatts   json.Number `json:"potentialsolarwatts,omitempty"`
	SolarJoules           json.Number `json:"solarjoules,omitempty"`
	UV                    json.Number `json:"uv,omitempty"`
	Radiation             json.Number `json:"radiation,omitempty"`
	StormRain             json.Number `json:"stormrain,omitempty"`
	DayRain               json.Number `json:"dayrain,omitempty"`
	MonthRain             json.Number `json:"monthrain,omitempty"`
	YearRain              json.Number `json:"yearrain,omitempty"`
	Barometer             json.Number `json:"bar,omitempty"`
	WindSpeed             json.Number `json:"winds,omitempty"`
	WindDirection         json.Number `json:"windd,omitempty"`
	CardinalDirection     string      `json:"windcard,omitempty"`
	RainfallDay           json.Number `json:"rainday,omitempty"`
	WindChill             json.Number `json:"windch,omitempty"`
	HeatIndex             json.Number `json:"heatidx,omitempty"`
	InsideTemperature     json.Number `json:"itemp,omitempty"`
	InsideHumidity        json.Number `json:"ihum,omitempty"`
	ConsBatteryVoltage    json.Number `json:"consbatteryvoltage,omitempty"`
	StationBatteryVoltage json.Number `json:"stationbatteryvoltage,omitempty"`
	SnowDepth             json.Number `json:"snowdepth,omitempty"`
	SnowDistance          json.Number `json:"snowdistance,omitempty"`
	ExtraFloat1           json.Number `json:"extrafloat1,omitempty"`
	ExtraFloat2           json.Number `json:"extrafloat2,omitempty"`
	ExtraFloat3           json.Number `json:"extrafloat3,omitempty"`
	ExtraFloat4           json.Number `json:"extrafloat4,omitempty"`
	ExtraFloat5           json.Number `json:"extrafloat5,omitempty"`
	ExtraFloat6           json.Number `json:"extrafloat6,omitempty"`
	ExtraFloat7           json.Number `json:"extrafloat7,omitempty"`
	ExtraFloat8           json.Number `json:"extrafloat8,omitempty"`
	ExtraFloat9           json.Number `json:"extrafloat9,omitempty"`
	ExtraFloat10          json.Number `json:"extrafloat10,omitempty"`
	ExtraText1            string      `json:"extratext1,omitempty"`
	ExtraText2            string      `json:"extratext2,omitempty"`
	ExtraText3            string      `json:"extratext3,omitempty"`
	ExtraText4            string      `json:"extratext4,omitempty"`
	ExtraText5            string      `json:"extratext5,omitempty"`
	ExtraText6            string      `json:"extratext6,omitempty"`
	ExtraText7            string      `json:"extratext7,omitempty"`
	ExtraText8            string      `json:"extratext8,omitempty"`
	ExtraText9            string      `json:"extratext9,omitempty"`
	ExtraText10           string      `json:"extratext10,omitempty"`
}

// SnowReading represents snow data for JSON output
type SnowReading struct {
	StationName  string  `json:"stationname"`
	SnowDepth    float32 `json:"snowdepth"`
	SnowToday    float32 `json:"snowtoday"`
	SnowLast24   float32 `json:"snowlast24"`
	SnowLast72   float32 `json:"snowlast72"`
	SnowSeason   float32 `json:"snowseason"`
	SnowStorm    float32 `json:"snowstorm"`
	SnowfallRate float32 `json:"snowfallrate"`
}

// SnowSeasonReading represents seasonal snow data
type SnowSeasonReading struct {
	StationName         string  `json:"stationname"`
	TotalSeasonSnowfall float32 `json:"totalseasonsnowfall"`
}

// SnowDeltaResult represents snow delta calculation result
type SnowDeltaResult struct {
	Snowfall float32
}

// SnowAllCalculationsResult represents all snow calculations in a single query
type SnowAllCalculationsResult struct {
	SnowSinceMidnight float32 `gorm:"column:snow_since_midnight"`
	SnowLast24        float32 `gorm:"column:snow_last_24h"`
	SnowLast72        float32 `gorm:"column:snow_last_72h"`
	SnowSeason        float32 `gorm:"column:snow_season"`
	SnowStorm         float32 `gorm:"column:snow_storm"`
}

// Utility functions

// float32ToJSONNumber converts a float32 to a JSON number, handling NaN and Inf values
func float32ToJSONNumber(f float32) json.Number {
	var s string
	if f == float32(int32(f)) {
		s = fmt.Sprintf("%.1f", f) // 1 decimal if integer
	} else {
		s = fmt.Sprint(f)
	}
	return json.Number(s)
}

// mmToInches converts millimeters to inches
func mmToInches(mm float32) float32 {
	return mm / 25.4
}

// headingToCardinalDirection converts a wind direction heading to a cardinal direction
func headingToCardinalDirection(f float32) string {
	cardDirections := []string{"N", "NNE", "NE", "ENE",
		"E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW",
		"W", "WNW", "NW", "NNW"}

	cardIndex := int((float32(f) + float32(11.25)) / float32(22.5))
	return cardDirections[cardIndex%16]
}

// StationData represents weather station information for the portal
type StationData struct {
	Name      string              `json:"name"`
	Type      string              `json:"type"`
	Latitude  float64             `json:"latitude"`
	Longitude float64             `json:"longitude"`
	Enabled   bool                `json:"enabled"`
	Website   *StationWebsiteData `json:"website,omitempty"`
}

// StationWebsiteData represents weather website information for a station
type StationWebsiteData struct {
	Name      string `json:"name"`
	Hostname  string `json:"hostname"`
	PageTitle string `json:"page_title"`
	Protocol  string `json:"protocol"`
	Port      int    `json:"port"`
}
