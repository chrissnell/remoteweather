package restserver

import (
	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/types"
	weatherapps "github.com/chrissnell/remoteweather/protocols/weatherapps"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// calculateFeelsLikeTemp returns the feels-like temperature
// Uses wind chill for cold conditions, heat index for hot conditions, or actual temp otherwise
func calculateFeelsLikeTemp(outTemp, outHumidity, windSpeed float32) float32 {
	// Wind chill applies when temp <= 50°F and wind >= 3 mph
	if outTemp <= 50 && windSpeed >= 3 {
		return controllers.CalculateWindChill(outTemp, windSpeed)
	}
	// Heat index applies when temp >= 80°F
	if outTemp >= 80 {
		return controllers.CalculateHeatIndex(outTemp, outHumidity)
	}
	// Otherwise, just return the actual temperature
	return outTemp
}

// calculateSkyCondition determines sky conditions based on solar radiation
// Uses the same algorithm as the website JavaScript (weather-utils.js)
// If no radiation sensor, returns SKY_UNKNOWN
func calculateSkyCondition(radiation, potentialSolarWatts float32) weatherapps.SkyCondition {
	// If potential solar is very low, it's night (matches website: maxValue < 10)
	// Check this first since potentialSolarWatts is 0 at night
	if potentialSolarWatts < 10 {
		return weatherapps.SkyCondition_SKY_NIGHT
	}

	// If we don't have radiation data, return unknown
	if radiation <= 0 {
		return weatherapps.SkyCondition_SKY_UNKNOWN
	}

	// Calculate the percentage of potential solar radiation
	percentage := (radiation / potentialSolarWatts) * 100

	// Classify sky conditions based on percentage (matches website thresholds)
	switch {
	case percentage >= 80:
		return weatherapps.SkyCondition_SKY_CLEAR // Sunny
	case percentage >= 40:
		return weatherapps.SkyCondition_SKY_PARTLY_CLOUDY
	default:
		return weatherapps.SkyCondition_SKY_CLOUDY
	}
}

// transformBucketReadingToWeatherApps converts a database reading to weatherapps.WeatherReading
// This function creates the base reading from database values.
// Calculated fields (rain totals, wind gust, etc.) must be added by the caller.
func transformBucketReadingToWeatherApps(r *types.BucketReading) *weatherapps.WeatherReading {
	feelsLike := calculateFeelsLikeTemp(r.OutTemp, r.OutHumidity, r.WindSpeed)
	skyCondition := calculateSkyCondition(r.Radiation, r.PotentialSolarWatts)
	cardinalDir := headingToCardinalDirection(r.WindDir)

	reading := &weatherapps.WeatherReading{
		ReadingTimestamp:      timestamppb.New(r.Timestamp),
		StationName:           r.StationName,
		StationType:           r.StationType,
		Temperature:           r.OutTemp,
		ExtraTemp1:            r.ExtraTemp1,
		ExtraTemp2:            r.ExtraTemp2,
		ExtraTemp3:            r.ExtraTemp3,
		ExtraTemp4:            r.ExtraTemp4,
		ExtraTemp5:            r.ExtraTemp5,
		ExtraTemp6:            r.ExtraTemp6,
		ExtraTemp7:            r.ExtraTemp7,
		SoilTemp1:             r.SoilTemp1,
		SoilTemp2:             r.SoilTemp2,
		SoilTemp3:             r.SoilTemp3,
		SoilTemp4:             r.SoilTemp4,
		LeafTemp1:             r.LeafTemp1,
		LeafTemp2:             r.LeafTemp2,
		LeafTemp3:             r.LeafTemp3,
		LeafTemp4:             r.LeafTemp4,
		FeelsLikeTemp:         feelsLike,
		Humidity:              r.OutHumidity,
		ExtraHumidity1:        r.ExtraHumidity1,
		ExtraHumidity2:        r.ExtraHumidity2,
		ExtraHumidity3:        r.ExtraHumidity3,
		ExtraHumidity4:        r.ExtraHumidity4,
		ExtraHumidity5:        r.ExtraHumidity5,
		ExtraHumidity6:        r.ExtraHumidity6,
		ExtraHumidity7:        r.ExtraHumidity7,
		Barometer:             r.Barometer,
		WindSpeed:             r.WindSpeed,
		WindDirection:         r.WindDir,
		CardinalDirection:     cardinalDir,
		RainIncremental:       r.RainIncremental,
		RainToday:             r.DayRain,
		RainMonth:             r.MonthRain,
		RainYear:              r.YearRain,
		SolarWatts:            r.SolarWatts,
		PotentialSolarWatts:   r.PotentialSolarWatts,
		SolarJoules:           r.SolarJoules,
		Uv:                    r.UV,
		Radiation:             r.Radiation,
		SkyConditions:         skyCondition,
		SnowDepth:             r.SnowDepth,
		Pm25:                  r.PM25,
		Co2:                   r.CO2,
		AqiPM25:               getOrCalculateAQIPM25(r.Reading),
		AqiPM10:               getOrCalculateAQIPM10(r.Reading),
		ConsBatteryVoltage:    r.ConsBatteryVoltage,
		StationBatteryVoltage: r.StationBatteryVoltage,
		ExtraFloat1:           r.ExtraFloat1,
		ExtraFloat2:           r.ExtraFloat2,
		ExtraFloat3:           r.ExtraFloat3,
		ExtraFloat4:           r.ExtraFloat4,
		ExtraFloat5:           r.ExtraFloat5,
		ExtraFloat6:           r.ExtraFloat6,
		ExtraFloat7:           r.ExtraFloat7,
		ExtraFloat8:           r.ExtraFloat8,
		ExtraFloat9:           r.ExtraFloat9,
		ExtraFloat10:          r.ExtraFloat10,
		ExtraText1:            r.ExtraText1,
		ExtraText2:            r.ExtraText2,
		ExtraText3:            r.ExtraText3,
		ExtraText4:            r.ExtraText4,
		ExtraText5:            r.ExtraText5,
		ExtraText6:            r.ExtraText6,
		ExtraText7:            r.ExtraText7,
		ExtraText8:            r.ExtraText8,
		ExtraText9:            r.ExtraText9,
		ExtraText10:           r.ExtraText10,
	}

	return reading
}
