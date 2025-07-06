// Package grpcutil provides shared utilities for gRPC-based weather data services.
package grpcutil

import (
	"fmt"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	weather "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DeviceManager provides device configuration management for gRPC services
type DeviceManager struct {
	Devices     []config.DeviceData
	DeviceNames map[string]bool // device name -> exists (for fast O(1) lookups)
}

// NewDeviceManager creates a new device manager from configuration
func NewDeviceManager(devices []config.DeviceData) *DeviceManager {
	dm := &DeviceManager{
		Devices:     devices,
		DeviceNames: make(map[string]bool),
	}

	// Build device name map for fast O(1) lookups
	for _, device := range devices {
		dm.DeviceNames[device.Name] = true
	}

	return dm
}

// ValidateStationExists checks if a station name exists in the configuration
func (dm *DeviceManager) ValidateStationExists(stationName string) bool {
	if stationName == "" {
		return false
	}
	return dm.DeviceNames[stationName]
}

// GetSnowBaseDistance returns the snow base distance for a given device
func (dm *DeviceManager) GetSnowBaseDistance(deviceName string) float32 {
	if deviceName == "" {
		return 0.0
	}

	for _, device := range dm.Devices {
		if device.Name == deviceName {
			return float32(device.BaseSnowDistance)
		}
	}
	return 0.0
}

// TransformBucketReadings converts database bucket readings to protobuf WeatherReading slice
func TransformBucketReadings(dbReadings *[]types.BucketReading) []*weather.WeatherReading {
	// Pre-allocate slice with exact capacity to avoid multiple reallocations
	grpcReadings := make([]*weather.WeatherReading, 0, len(*dbReadings))

	for _, r := range *dbReadings {
		grpcReadings = append(grpcReadings, &weather.WeatherReading{
			ReadingTimestamp: timestamppb.New(r.Bucket),
			StationName:      r.StationName,
			StationType:      r.StationType,

			// Primary environmental readings
			Barometer:          r.Barometer,
			InsideTemperature:  r.InTemp,
			InsideHumidity:     r.InHumidity,
			OutsideTemperature: r.OutTemp,
			OutsideHumidity:    r.OutHumidity,

			// Wind measurements
			WindSpeed:     r.WindSpeed,
			WindSpeed10:   r.WindSpeed10,
			WindDirection: r.WindDir,
			WindChill:     r.WindChill,
			HeatIndex:     r.HeatIndex,

			// Additional temperature sensors
			ExtraTemp1: r.ExtraTemp1,
			ExtraTemp2: r.ExtraTemp2,
			ExtraTemp3: r.ExtraTemp3,
			ExtraTemp4: r.ExtraTemp4,
			ExtraTemp5: r.ExtraTemp5,
			ExtraTemp6: r.ExtraTemp6,
			ExtraTemp7: r.ExtraTemp7,

			// Soil temperature sensors
			SoilTemp1: r.SoilTemp1,
			SoilTemp2: r.SoilTemp2,
			SoilTemp3: r.SoilTemp3,
			SoilTemp4: r.SoilTemp4,

			// Leaf temperature sensors
			LeafTemp1: r.LeafTemp1,
			LeafTemp2: r.LeafTemp2,
			LeafTemp3: r.LeafTemp3,
			LeafTemp4: r.LeafTemp4,

			// Additional humidity sensors
			ExtraHumidity1: r.ExtraHumidity1,
			ExtraHumidity2: r.ExtraHumidity2,
			ExtraHumidity3: r.ExtraHumidity3,
			ExtraHumidity4: r.ExtraHumidity4,
			ExtraHumidity5: r.ExtraHumidity5,
			ExtraHumidity6: r.ExtraHumidity6,
			ExtraHumidity7: r.ExtraHumidity7,

			// Rain measurements
			RainRate:        r.RainRate,
			RainIncremental: r.RainIncremental,
			StormRain:       r.StormRain,
			StormStart:      timestamppb.New(r.StormStart),
			DayRain:         r.DayRain,
			MonthRain:       r.MonthRain,
			YearRain:        r.YearRain,

			// Solar measurements
			SolarWatts:          r.SolarWatts,
			PotentialSolarWatts: r.PotentialSolarWatts,
			SolarJoules:         r.SolarJoules,
			Uv:                  r.UV,
			Radiation:           r.Radiation,

			// Evapotranspiration
			DayET:   r.DayET,
			MonthET: r.MonthET,
			YearET:  r.YearET,

			// Soil moisture sensors
			SoilMoisture1: r.SoilMoisture1,
			SoilMoisture2: r.SoilMoisture2,
			SoilMoisture3: r.SoilMoisture3,
			SoilMoisture4: r.SoilMoisture4,

			// Leaf wetness sensors
			LeafWetness1: r.LeafWetness1,
			LeafWetness2: r.LeafWetness2,
			LeafWetness3: r.LeafWetness3,
			LeafWetness4: r.LeafWetness4,

			// Alarm states
			InsideAlarm:    uint32(r.InsideAlarm),
			RainAlarm:      uint32(r.RainAlarm),
			OutsideAlarm1:  uint32(r.OutsideAlarm1),
			OutsideAlarm2:  uint32(r.OutsideAlarm2),
			ExtraAlarm1:    uint32(r.ExtraAlarm1),
			ExtraAlarm2:    uint32(r.ExtraAlarm2),
			ExtraAlarm3:    uint32(r.ExtraAlarm3),
			ExtraAlarm4:    uint32(r.ExtraAlarm4),
			ExtraAlarm5:    uint32(r.ExtraAlarm5),
			ExtraAlarm6:    uint32(r.ExtraAlarm6),
			ExtraAlarm7:    uint32(r.ExtraAlarm7),
			ExtraAlarm8:    uint32(r.ExtraAlarm8),
			SoilLeafAlarm1: uint32(r.SoilLeafAlarm1),
			SoilLeafAlarm2: uint32(r.SoilLeafAlarm2),
			SoilLeafAlarm3: uint32(r.SoilLeafAlarm3),
			SoilLeafAlarm4: uint32(r.SoilLeafAlarm4),

			// Battery and power status
			TxBatteryStatus:       uint32(r.TxBatteryStatus),
			ConsBatteryVoltage:    r.ConsBatteryVoltage,
			StationBatteryVoltage: r.StationBatteryVoltage,

			// Forecast information
			ForecastIcon: uint32(r.ForecastIcon),
			ForecastRule: uint32(r.ForecastRule),

			// Astronomical data
			Sunrise: timestamppb.New(r.Sunrise),
			Sunset:  timestamppb.New(r.Sunset),

			// Snow measurements - SnowDepth is calculated by the database query
			SnowDistance: r.SnowDistance,
			SnowDepth:    r.SnowDepth,

			// Extended float fields
			ExtraFloat1:  r.ExtraFloat1,
			ExtraFloat2:  r.ExtraFloat2,
			ExtraFloat3:  r.ExtraFloat3,
			ExtraFloat4:  r.ExtraFloat4,
			ExtraFloat5:  r.ExtraFloat5,
			ExtraFloat6:  r.ExtraFloat6,
			ExtraFloat7:  r.ExtraFloat7,
			ExtraFloat8:  r.ExtraFloat8,
			ExtraFloat9:  r.ExtraFloat9,
			ExtraFloat10: r.ExtraFloat10,

			// Extended text fields
			ExtraText1:  r.ExtraText1,
			ExtraText2:  r.ExtraText2,
			ExtraText3:  r.ExtraText3,
			ExtraText4:  r.ExtraText4,
			ExtraText5:  r.ExtraText5,
			ExtraText6:  r.ExtraText6,
			ExtraText7:  r.ExtraText7,
			ExtraText8:  r.ExtraText8,
			ExtraText9:  r.ExtraText9,
			ExtraText10: r.ExtraText10,
		})
	}

	return grpcReadings
}

// TransformReading converts a single types.Reading to protobuf WeatherReading
func TransformReading(r types.Reading) *weather.WeatherReading {
	return &weather.WeatherReading{
		ReadingTimestamp: timestamppb.New(r.Timestamp),
		StationName:      r.StationName,
		StationType:      r.StationType,

		// Primary environmental readings
		Barometer:          r.Barometer,
		InsideTemperature:  r.InTemp,
		InsideHumidity:     r.InHumidity,
		OutsideTemperature: r.OutTemp,
		OutsideHumidity:    r.OutHumidity,

		// Wind measurements
		WindSpeed:     r.WindSpeed,
		WindSpeed10:   r.WindSpeed10,
		WindDirection: r.WindDir,
		WindChill:     r.WindChill,
		HeatIndex:     r.HeatIndex,

		// Additional temperature sensors
		ExtraTemp1: r.ExtraTemp1,
		ExtraTemp2: r.ExtraTemp2,
		ExtraTemp3: r.ExtraTemp3,
		ExtraTemp4: r.ExtraTemp4,
		ExtraTemp5: r.ExtraTemp5,
		ExtraTemp6: r.ExtraTemp6,
		ExtraTemp7: r.ExtraTemp7,

		// Soil temperature sensors
		SoilTemp1: r.SoilTemp1,
		SoilTemp2: r.SoilTemp2,
		SoilTemp3: r.SoilTemp3,
		SoilTemp4: r.SoilTemp4,

		// Leaf temperature sensors
		LeafTemp1: r.LeafTemp1,
		LeafTemp2: r.LeafTemp2,
		LeafTemp3: r.LeafTemp3,
		LeafTemp4: r.LeafTemp4,

		// Additional humidity sensors
		ExtraHumidity1: r.ExtraHumidity1,
		ExtraHumidity2: r.ExtraHumidity2,
		ExtraHumidity3: r.ExtraHumidity3,
		ExtraHumidity4: r.ExtraHumidity4,
		ExtraHumidity5: r.ExtraHumidity5,
		ExtraHumidity6: r.ExtraHumidity6,
		ExtraHumidity7: r.ExtraHumidity7,

		// Rain measurements
		RainRate:        r.RainRate,
		RainIncremental: r.RainIncremental,
		StormRain:       r.StormRain,
		StormStart:      timestamppb.New(r.StormStart),
		DayRain:         r.DayRain,
		MonthRain:       r.MonthRain,
		YearRain:        r.YearRain,

		// Solar measurements
		SolarWatts:          r.SolarWatts,
		PotentialSolarWatts: r.PotentialSolarWatts,
		SolarJoules:         r.SolarJoules,
		Uv:                  r.UV,
		Radiation:           r.Radiation,

		// Evapotranspiration
		DayET:   r.DayET,
		MonthET: r.MonthET,
		YearET:  r.YearET,

		// Soil moisture sensors
		SoilMoisture1: r.SoilMoisture1,
		SoilMoisture2: r.SoilMoisture2,
		SoilMoisture3: r.SoilMoisture3,
		SoilMoisture4: r.SoilMoisture4,

		// Leaf wetness sensors
		LeafWetness1: r.LeafWetness1,
		LeafWetness2: r.LeafWetness2,
		LeafWetness3: r.LeafWetness3,
		LeafWetness4: r.LeafWetness4,

		// Alarm states
		InsideAlarm:    uint32(r.InsideAlarm),
		RainAlarm:      uint32(r.RainAlarm),
		OutsideAlarm1:  uint32(r.OutsideAlarm1),
		OutsideAlarm2:  uint32(r.OutsideAlarm2),
		ExtraAlarm1:    uint32(r.ExtraAlarm1),
		ExtraAlarm2:    uint32(r.ExtraAlarm2),
		ExtraAlarm3:    uint32(r.ExtraAlarm3),
		ExtraAlarm4:    uint32(r.ExtraAlarm4),
		ExtraAlarm5:    uint32(r.ExtraAlarm5),
		ExtraAlarm6:    uint32(r.ExtraAlarm6),
		ExtraAlarm7:    uint32(r.ExtraAlarm7),
		ExtraAlarm8:    uint32(r.ExtraAlarm8),
		SoilLeafAlarm1: uint32(r.SoilLeafAlarm1),
		SoilLeafAlarm2: uint32(r.SoilLeafAlarm2),
		SoilLeafAlarm3: uint32(r.SoilLeafAlarm3),
		SoilLeafAlarm4: uint32(r.SoilLeafAlarm4),

		// Battery and power status
		TxBatteryStatus:       uint32(r.TxBatteryStatus),
		ConsBatteryVoltage:    r.ConsBatteryVoltage,
		StationBatteryVoltage: r.StationBatteryVoltage,

		// Forecast information
		ForecastIcon: uint32(r.ForecastIcon),
		ForecastRule: uint32(r.ForecastRule),

		// Astronomical data
		Sunrise: timestamppb.New(r.Sunrise),
		Sunset:  timestamppb.New(r.Sunset),

		// Snow measurements
		SnowDistance: r.SnowDistance,
		SnowDepth:    r.SnowDepth,

		// Extended float fields
		ExtraFloat1:  r.ExtraFloat1,
		ExtraFloat2:  r.ExtraFloat2,
		ExtraFloat3:  r.ExtraFloat3,
		ExtraFloat4:  r.ExtraFloat4,
		ExtraFloat5:  r.ExtraFloat5,
		ExtraFloat6:  r.ExtraFloat6,
		ExtraFloat7:  r.ExtraFloat7,
		ExtraFloat8:  r.ExtraFloat8,
		ExtraFloat9:  r.ExtraFloat9,
		ExtraFloat10: r.ExtraFloat10,

		// Extended text fields
		ExtraText1:  r.ExtraText1,
		ExtraText2:  r.ExtraText2,
		ExtraText3:  r.ExtraText3,
		ExtraText4:  r.ExtraText4,
		ExtraText5:  r.ExtraText5,
		ExtraText6:  r.ExtraText6,
		ExtraText7:  r.ExtraText7,
		ExtraText8:  r.ExtraText8,
		ExtraText9:  r.ExtraText9,
		ExtraText10: r.ExtraText10,
	}
}

// ValidateStationRequest validates that a station name is provided and exists
func ValidateStationRequest(stationName string, deviceManager *DeviceManager) error {
	if stationName == "" {
		return fmt.Errorf("stationName is required")
	}
	if !deviceManager.ValidateStationExists(stationName) {
		return fmt.Errorf("station not found: %s", stationName)
	}
	return nil
}
