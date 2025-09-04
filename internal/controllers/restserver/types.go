package restserver

import (
	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)


// AerisWeatherForecastRecord represents forecast data from Aeris Weather
type AerisWeatherForecastRecord struct {
	gorm.Model

	StationID         int          `gorm:"uniqueIndex:idx_station_span,not null"`
	ForecastSpanHours int16        `gorm:"uniqueIndex:idx_station_span,not null"`
	Location          string       `gorm:"not null"`
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
	// Using float32 for all numeric fields - proper types for both JSON and MessagePack
	OutsideTemperature  float32 `json:"otemp,omitempty"`
	ExtraTemp1          float32 `json:"extratemp1,omitempty"`
	ExtraTemp2          float32 `json:"extratemp2,omitempty"`
	ExtraTemp3          float32 `json:"extratemp3,omitempty"`
	ExtraTemp4          float32 `json:"extratemp4,omitempty"`
	ExtraTemp5          float32 `json:"extratemp5,omitempty"`
	ExtraTemp6          float32 `json:"extratemp6,omitempty"`
	ExtraTemp7          float32 `json:"extratemp7,omitempty"`
	SoilTemp1           float32 `json:"soiltemp1,omitempty"`
	SoilTemp2           float32 `json:"soiltemp2,omitempty"`
	SoilTemp3           float32 `json:"soiltemp3,omitempty"`
	SoilTemp4           float32 `json:"soiltemp4,omitempty"`
	LeafTemp1           float32 `json:"leaftemp1,omitempty"`
	LeafTemp2           float32 `json:"leaftemp2,omitempty"`
	LeafTemp3           float32 `json:"leaftemp3,omitempty"`
	LeafTemp4           float32 `json:"leaftemp4,omitempty"`
	OutHumidity         float32 `json:"outhumidity,omitempty"`
	ExtraHumidity1      float32 `json:"extrahumidity1,omitempty"`
	ExtraHumidity2      float32 `json:"extrahumidity2,omitempty"`
	ExtraHumidity3      float32 `json:"extrahumidity3,omitempty"`
	ExtraHumidity4      float32 `json:"extrahumidity4,omitempty"`
	ExtraHumidity5      float32 `json:"extrahumidity5,omitempty"`
	ExtraHumidity6      float32 `json:"extrahumidity6,omitempty"`
	ExtraHumidity7      float32 `json:"extrahumidity7,omitempty"`
	OutsideHumidity     float32 `json:"ohum,omitempty"`
	RainRate            float32 `json:"rainrate,omitempty"`
	RainIncremental     float32 `json:"rainincremental,omitempty"`
	PeriodRain          float32 `json:"period_rain,omitempty"`
	SolarWatts          float32 `json:"solarwatts,omitempty"`
	PotentialSolarWatts float32 `json:"potentialsolarwatts,omitempty"`
	SolarJoules         float32 `json:"solarjoules,omitempty"`
	UV                  float32 `json:"uv,omitempty"`
	Radiation           float32 `json:"radiation,omitempty"`
	StormRain           float32 `json:"stormrain,omitempty"`
	DayRain             float32 `json:"dayrain,omitempty"`
	MonthRain           float32 `json:"monthrain,omitempty"`
	YearRain            float32 `json:"yearrain,omitempty"`
	Barometer           float32 `json:"bar,omitempty"`
	// New rainfall total fields
	Rainfall24h           float32 `json:"rainfall24h,omitempty"`
	Rainfall48h           float32 `json:"rainfall48h,omitempty"`
	Rainfall72h           float32 `json:"rainfall72h,omitempty"`
	RainfallStorm         float32 `json:"rainfallstorm,omitempty"`
	WindSpeed             float32 `json:"winds,omitempty"`
	WindGust              float32 `json:"windgust,omitempty"`
	WindDirection         float32 `json:"windd,omitempty"`
	CardinalDirection     string  `json:"windcard,omitempty"`
	RainfallDay           float32 `json:"rainday,omitempty"`
	WindChill             float32 `json:"windch,omitempty"`
	HeatIndex             float32 `json:"heatidx,omitempty"`
	InsideTemperature     float32 `json:"itemp,omitempty"`
	InsideHumidity        float32 `json:"ihum,omitempty"`
	ConsBatteryVoltage    float32 `json:"consbatteryvoltage,omitempty"`
	StationBatteryVoltage float32 `json:"stationbatteryvoltage,omitempty"`
	SnowDepth             float32 `json:"snowdepth,omitempty"`
	SnowDistance          float32 `json:"snowdistance,omitempty"`
	PM25                  float32 `json:"pm25"`
	CO2                   float32 `json:"co2"`
	AQIPM25AQIN           float32 `json:"aqi_pm25_aqin"`
	AQIPM10AQIN           float32 `json:"aqi_pm10_aqin"`
	ExtraFloat1           float32 `json:"extrafloat1,omitempty"`
	ExtraFloat2           float32 `json:"extrafloat2,omitempty"`
	ExtraFloat3           float32 `json:"extrafloat3,omitempty"`
	ExtraFloat4           float32 `json:"extrafloat4,omitempty"`
	ExtraFloat5           float32 `json:"extrafloat5,omitempty"`
	ExtraFloat6           float32 `json:"extrafloat6,omitempty"`
	ExtraFloat7           float32 `json:"extrafloat7,omitempty"`
	ExtraFloat8           float32 `json:"extrafloat8,omitempty"`
	ExtraFloat9           float32 `json:"extrafloat9,omitempty"`
	ExtraFloat10          float32 `json:"extrafloat10,omitempty"`
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
	ID        int                 `json:"id"`
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
