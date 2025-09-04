package restserver

import (
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/aqi"
)

// transformSpanReadings converts database readings to WeatherReading slice for JSON output
func (h *Handlers) transformSpanReadings(dbReadings *[]types.BucketReading) []*WeatherReading {
	// Pre-allocate slice with exact capacity to avoid multiple reallocations
	wr := make([]*WeatherReading, 0, len(*dbReadings))

	for _, r := range *dbReadings {
		wr = append(wr, &WeatherReading{
			StationName:           r.StationName,
			StationType:           r.StationType,
			ReadingTimestamp:      r.Bucket.UnixMilli(),
			OutsideTemperature:    r.OutTemp,
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
			OutHumidity:           r.OutHumidity,
			ExtraHumidity1:        r.ExtraHumidity1,
			ExtraHumidity2:        r.ExtraHumidity2,
			ExtraHumidity3:        r.ExtraHumidity3,
			ExtraHumidity4:        r.ExtraHumidity4,
			ExtraHumidity5:        r.ExtraHumidity5,
			ExtraHumidity6:        r.ExtraHumidity6,
			ExtraHumidity7:        r.ExtraHumidity7,
			OutsideHumidity:       r.OutHumidity,
			RainRate:              r.RainRate,
			RainIncremental:       r.RainIncremental,
			PeriodRain:            r.PeriodRain,
			SolarWatts:            r.SolarWatts,
			PotentialSolarWatts:   r.PotentialSolarWatts,
			SolarJoules:           r.SolarJoules,
			UV:                    r.UV,
			Radiation:             r.Radiation,
			StormRain:             r.StormRain,
			DayRain:               r.DayRain,
			MonthRain:             r.MonthRain,
			YearRain:              r.YearRain,
			Barometer:             r.Barometer,
			WindSpeed:             r.WindSpeed,
			WindDirection:         r.WindDir,
			CardinalDirection:     headingToCardinalDirection(r.WindDir),
			RainfallDay:           r.DayRain,
			WindChill:             r.WindChill,
			HeatIndex:             r.HeatIndex,
			InsideTemperature:     r.InTemp,
			InsideHumidity:        r.InHumidity,
			ConsBatteryVoltage:    r.ConsBatteryVoltage,
			StationBatteryVoltage: r.StationBatteryVoltage,
			SnowDepth:             mmToInches(r.SnowDepth),
			SnowDistance:          r.SnowDistance,
			PM25:                  r.PM25,
			CO2:                   r.CO2,
			AQIPM25AQIN:           float32(getOrCalculateAQIPM25(r.Reading)),
			AQIPM10AQIN:           float32(getOrCalculateAQIPM10(r.Reading)),
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
		})
	}

	return wr
}

// transformLatestReadings converts database readings to a single WeatherReading for JSON output
func (h *Handlers) transformLatestReadings(dbReadings *[]types.BucketReading) *WeatherReading {
	var latest types.BucketReading

	if len(*dbReadings) > 0 {
		latest = (*dbReadings)[0]
	} else {
		return &WeatherReading{}
	}

	reading := WeatherReading{
		StationName:           latest.StationName,
		StationType:           latest.StationType,
		ReadingTimestamp:      latest.Timestamp.UnixMilli(),
		OutsideTemperature:    latest.OutTemp,
		ExtraTemp1:            latest.ExtraTemp1,
		ExtraTemp2:            latest.ExtraTemp2,
		ExtraTemp3:            latest.ExtraTemp3,
		ExtraTemp4:            latest.ExtraTemp4,
		ExtraTemp5:            latest.ExtraTemp5,
		ExtraTemp6:            latest.ExtraTemp6,
		ExtraTemp7:            latest.ExtraTemp7,
		SoilTemp1:             latest.SoilTemp1,
		SoilTemp2:             latest.SoilTemp2,
		SoilTemp3:             latest.SoilTemp3,
		SoilTemp4:             latest.SoilTemp4,
		LeafTemp1:             latest.LeafTemp1,
		LeafTemp2:             latest.LeafTemp2,
		LeafTemp3:             latest.LeafTemp3,
		LeafTemp4:             latest.LeafTemp4,
		OutHumidity:           latest.OutHumidity,
		ExtraHumidity1:        latest.ExtraHumidity1,
		ExtraHumidity2:        latest.ExtraHumidity2,
		ExtraHumidity3:        latest.ExtraHumidity3,
		ExtraHumidity4:        latest.ExtraHumidity4,
		ExtraHumidity5:        latest.ExtraHumidity5,
		ExtraHumidity6:        latest.ExtraHumidity6,
		ExtraHumidity7:        latest.ExtraHumidity7,
		OutsideHumidity:       latest.OutHumidity,
		RainRate:              latest.RainRate,
		RainIncremental:       latest.RainIncremental,
		PeriodRain:            latest.PeriodRain,
		SolarWatts:            latest.SolarWatts,
		PotentialSolarWatts:   latest.PotentialSolarWatts,
		SolarJoules:           latest.SolarJoules,
		UV:                    latest.UV,
		Radiation:             latest.Radiation,
		StormRain:             latest.StormRain,
		DayRain:               latest.DayRain,
		MonthRain:             latest.MonthRain,
		YearRain:              latest.YearRain,
		Barometer:             latest.Barometer,
		WindSpeed:             latest.WindSpeed,
		WindDirection:         latest.WindDir,
		CardinalDirection:     headingToCardinalDirection(latest.WindDir),
		RainfallDay:           latest.DayRain,
		WindChill:             latest.WindChill,
		HeatIndex:             latest.HeatIndex,
		InsideTemperature:     latest.InTemp,
		InsideHumidity:        latest.InHumidity,
		ConsBatteryVoltage:    latest.ConsBatteryVoltage,
		StationBatteryVoltage: latest.StationBatteryVoltage,
		SnowDepth:             latest.SnowDepth,
		SnowDistance:          latest.SnowDistance,
		PM25:                  latest.PM25,
		CO2:                   latest.CO2,
		AQIPM25AQIN:           float32(getOrCalculateAQIPM25(latest.Reading)),
		AQIPM10AQIN:           float32(getOrCalculateAQIPM10(latest.Reading)),
		ExtraFloat1:           latest.ExtraFloat1,
		ExtraFloat2:           latest.ExtraFloat2,
		ExtraFloat3:           latest.ExtraFloat3,
		ExtraFloat4:           latest.ExtraFloat4,
		ExtraFloat5:           latest.ExtraFloat5,
		ExtraFloat6:           latest.ExtraFloat6,
		ExtraFloat7:           latest.ExtraFloat7,
		ExtraFloat8:           latest.ExtraFloat8,
		ExtraFloat9:           latest.ExtraFloat9,
		ExtraFloat10:          latest.ExtraFloat10,
		ExtraText1:            latest.ExtraText1,
		ExtraText2:            latest.ExtraText2,
		ExtraText3:            latest.ExtraText3,
		ExtraText4:            latest.ExtraText4,
		ExtraText5:            latest.ExtraText5,
		ExtraText6:            latest.ExtraText6,
		ExtraText7:            latest.ExtraText7,
		ExtraText8:            latest.ExtraText8,
		ExtraText9:            latest.ExtraText9,
		ExtraText10:           latest.ExtraText10,
	}
	return &reading
}

// getOrCalculateAQIPM25 returns the AQI PM2.5 value if available, otherwise calculates it from PM2.5
func getOrCalculateAQIPM25(r types.Reading) int32 {
	// If we already have an AQI value, use it
	if r.AQIPM25AQIN > 0 {
		return r.AQIPM25AQIN
	}
	// Otherwise calculate from PM2.5 if available
	if r.PM25 > 0 {
		return aqi.CalculatePM25(r.PM25)
	}
	return 0
}

// getOrCalculateAQIPM10 returns the AQI PM10 value if available, otherwise calculates it from PM10
func getOrCalculateAQIPM10(r types.Reading) int32 {
	// If we already have an AQI value, use it
	if r.AQIPM10AQIN > 0 {
		return r.AQIPM10AQIN
	}
	// PM10 isn't currently in the Reading struct, so return 0
	// TODO: Add PM10 field to Reading struct if PM10 sensors are added
	return 0
}
