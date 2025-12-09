package restserver

import (
	"math"
	"time"

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

// AerisWeatherAlertRecord represents alert data from Xweather
type AerisWeatherAlertRecord struct {
	gorm.Model

	StationID int          `gorm:"index:idx_station_alerts,not null"`
	AlertID   string       `gorm:"not null"`
	Location  string       `gorm:"not null"`
	IssuedAt  *time.Time   `gorm:"index"`
	BeginsAt  *time.Time   `gorm:"index"`
	ExpiresAt *time.Time   `gorm:"index"`
	Name      string       `gorm:"type:text"`
	Color     string       `gorm:"type:text"`
	Body      string       `gorm:"type:text"`
	BodyFull  string       `gorm:"type:text"`
	Data      pgtype.JSONB `gorm:"type:jsonb;not null"`
}

// TableName implements the GORM Tabler interface to specify the correct table name
func (AerisWeatherAlertRecord) TableName() string {
	return "aeris_weather_alerts"
}

// WeatherReading represents a weather reading for JSON output
type WeatherReading struct {
	StationName      string `json:"stationname"`
	StationType      string `json:"stationtype,omitempty"`
	ReadingTimestamp int64  `json:"ts"`
	// Using float32 for all numeric fields - proper types for both JSON and MessagePack
	// Note: omitempty removed from numeric fields to ensure zeros are included in JSON output
	OutsideTemperature  float32 `json:"otemp"`
	ExtraTemp1          float32 `json:"extratemp1"`
	ExtraTemp2          float32 `json:"extratemp2"`
	ExtraTemp3          float32 `json:"extratemp3"`
	ExtraTemp4          float32 `json:"extratemp4"`
	ExtraTemp5          float32 `json:"extratemp5"`
	ExtraTemp6          float32 `json:"extratemp6"`
	ExtraTemp7          float32 `json:"extratemp7"`
	SoilTemp1           float32 `json:"soiltemp1"`
	SoilTemp2           float32 `json:"soiltemp2"`
	SoilTemp3           float32 `json:"soiltemp3"`
	SoilTemp4           float32 `json:"soiltemp4"`
	LeafTemp1           float32 `json:"leaftemp1"`
	LeafTemp2           float32 `json:"leaftemp2"`
	LeafTemp3           float32 `json:"leaftemp3"`
	LeafTemp4           float32 `json:"leaftemp4"`
	OutHumidity         float32 `json:"outhumidity"`
	ExtraHumidity1      float32 `json:"extrahumidity1"`
	ExtraHumidity2      float32 `json:"extrahumidity2"`
	ExtraHumidity3      float32 `json:"extrahumidity3"`
	ExtraHumidity4      float32 `json:"extrahumidity4"`
	ExtraHumidity5      float32 `json:"extrahumidity5"`
	ExtraHumidity6      float32 `json:"extrahumidity6"`
	ExtraHumidity7      float32 `json:"extrahumidity7"`
	OutsideHumidity     float32 `json:"ohum"`
	RainRate            float32 `json:"rainrate"`
	RainIncremental     float32 `json:"rainincremental"`
	PeriodRain          float32 `json:"period_rain"`
	SolarWatts          float32 `json:"solarwatts"`
	PotentialSolarWatts float32 `json:"potentialsolarwatts"`
	SolarJoules         float32 `json:"solarjoules"`
	UV                  float32 `json:"uv"`
	Radiation           float32 `json:"radiation"`
	StormRain           float32 `json:"stormrain"`
	DayRain             float32 `json:"dayrain"`
	MonthRain           float32 `json:"monthrain"`
	YearRain            float32 `json:"yearrain"`
	Barometer           float32 `json:"bar"`
	// New rainfall total fields
	Rainfall24h           float32 `json:"rainfall24h"`
	Rainfall48h           float32 `json:"rainfall48h"`
	Rainfall72h           float32 `json:"rainfall72h"`
	RainfallStorm         float32 `json:"rainfallstorm"`
	WindSpeed             float32 `json:"winds"`
	WindGust              float32 `json:"windgust"`
	WindDirection         float32 `json:"windd"`
	CardinalDirection     string  `json:"windcard,omitempty"`
	RainfallDay           float32 `json:"rainday"`
	WindChill             float32 `json:"windch"`
	HeatIndex             float32 `json:"heatidx"`
	InsideTemperature     float32 `json:"itemp"`
	InsideHumidity        float32 `json:"ihum"`
	ConsBatteryVoltage    float32 `json:"consbatteryvoltage"`
	StationBatteryVoltage float32 `json:"stationbatteryvoltage"`
	SnowDepth             float32 `json:"snowdepth"`
	SnowDistance          float32 `json:"snowdistance"`
	PM25                  float32 `json:"pm25"`
	CO2                   float32 `json:"co2"`
	AQIPM25AQIN           float32 `json:"aqi_pm25_aqin"`
	AQIPM10AQIN           float32 `json:"aqi_pm10_aqin"`
	ExtraFloat1           float32 `json:"extrafloat1"`
	ExtraFloat2           float32 `json:"extrafloat2"`
	ExtraFloat3           float32 `json:"extrafloat3"`
	ExtraFloat4           float32 `json:"extrafloat4"`
	ExtraFloat5           float32 `json:"extrafloat5"`
	ExtraFloat6           float32 `json:"extrafloat6"`
	ExtraFloat7           float32 `json:"extrafloat7"`
	ExtraFloat8           float32 `json:"extrafloat8"`
	ExtraFloat9           float32 `json:"extrafloat9"`
	ExtraFloat10          float32 `json:"extrafloat10"`
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

// SnowCacheResult represents cached snow totals from snow_totals_cache table
type SnowCacheResult struct {
	StationName   string    `gorm:"column:stationname"`
	SnowMidnight  float32   `gorm:"column:snow_midnight"`
	Snow24h       float32   `gorm:"column:snow_24h"`
	Snow72h       float32   `gorm:"column:snow_72h"`
	SnowSeason    float32   `gorm:"column:snow_season"`
	BaseDistance  float32   `gorm:"column:base_distance"`
	ComputedAt    time.Time `gorm:"column:computed_at"`
}

// Utility functions

// mmToInches converts millimeters to inches
func mmToInches(mm float32) float32 {
	inches := mm / 25.4
	// Round to tenths place for clean display
	return float32(math.Round(float64(inches)*10) / 10)
}

// mmToInchesWithThreshold converts millimeters to inches with a minimum threshold
// to filter out sensor noise. Use for current readings, not cumulative totals.
func mmToInchesWithThreshold(mm float32) float32 {
	inches := mm / 25.4
	const threshold = 0.1
	
	// Filter out noise below threshold
	if inches < threshold {
		return 0.0
	}
	
	// Round to tenths place
	return float32(math.Round(float64(inches)*10) / 10)
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

// StationInfoItem represents a single weather station with its type
type StationInfoItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// StationInfoResponse represents the response for the /stationinfo endpoint
type StationInfoResponse struct {
	WebsiteName        string            `json:"website_name"`
	AboutText          string            `json:"about_text,omitempty"`
	Stations           []StationInfoItem `json:"stations"`
	WeatherDevice      *int              `json:"weather_device,omitempty"`
	SnowDevice         *string           `json:"snow_device,omitempty"`
	AirQualityDevice   *string           `json:"air_quality_device,omitempty"`
}

// Alert represents a weather alert for JSON output
type Alert struct {
	AlertID   string     `json:"alert_id"`
	StationID int        `json:"station_id"`
	Location  string     `json:"location"`
	Name      string     `json:"name"`
	Color     string     `json:"color"`
	Body      string     `json:"body"`
	BodyFull  string     `json:"body_full"`
	IssuedAt  *time.Time `json:"issued_at,omitempty"`
	BeginsAt  *time.Time `json:"begins_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
