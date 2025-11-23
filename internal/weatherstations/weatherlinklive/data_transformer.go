package weatherlinklive

import (
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
)

// transformToReading transforms WLL CurrentConditionsData to types.Reading
func (s *Station) transformToReading(data *CurrentConditionsData) *types.Reading {
	reading := &types.Reading{
		Timestamp:   time.Now(),
		StationName: s.config.Name,
		StationType: "weatherlink-live",
	}

	// Track occurrence counts for field assignment
	thCount := 0

	// Process each mapping in order (order matters for th sensors)
	for _, mapping := range s.mappings {
		condition := s.findCondition(data.Conditions, mapping)
		if condition == nil {
			continue
		}

		switch mapping.Type {
		case "th":
			s.applyTempHumidity(reading, condition, thCount)
			thCount++
		case "rain":
			s.applyRain(reading, condition)
		case "wind":
			s.applyWind(reading, condition)
		case "solar":
			s.applySolar(reading, condition)
		case "uv":
			s.applyUV(reading, condition)
		case "windchill":
			s.applyWindChill(reading, condition)
		case "thw":
			s.applyTHW(reading, condition, mapping.Options)
		case "thsw":
			s.applyTHSW(reading, condition, mapping.Options)
		case "baro":
			s.applyBarometer(reading, condition)
		case "th_indoor":
			s.applyIndoor(reading, condition)
		case "soil_temp":
			if mapping.Port != nil {
				s.applySoilTemp(reading, condition, *mapping.Port)
			}
		case "soil_moist":
			if mapping.Port != nil {
				s.applySoilMoisture(reading, condition, *mapping.Port)
			}
		case "leaf_wet":
			if mapping.Port != nil {
				s.applyLeafWetness(reading, condition, *mapping.Port)
			}
		case "battery":
			if mapping.TxID != nil {
				s.applyBattery(reading, condition, *mapping.TxID)
			}
		}
	}

	return reading
}

// findCondition finds the condition matching the sensor mapping
func (s *Station) findCondition(conditions []Condition, mapping SensorMapping) *Condition {
	for i := range conditions {
		condition := &conditions[i]

		// Match by TxID if specified
		if mapping.TxID != nil {
			if condition.TxID != nil && *condition.TxID == *mapping.TxID {
				return condition
			}
			continue
		}

		// For sensors without TxID (indoor, barometer), match by data structure type
		switch mapping.Type {
		case "baro":
			if condition.DataStructureType == DataStructureBarometer {
				return condition
			}
		case "th_indoor":
			if condition.DataStructureType == DataStructureBarometer {
				return condition
			}
		}
	}

	return nil
}

// applyTempHumidity applies temperature and humidity data
// Order matters: first th:X -> OutTemp/OutHumidity, second -> ExtraTemp1/ExtraHumid1, etc.
func (s *Station) applyTempHumidity(reading *types.Reading, condition *Condition, index int) {
	// Convert Fahrenheit to Celsius
	var tempC *float32
	if condition.Temp != nil {
		t := float32((*condition.Temp - 32) * 5 / 9)
		tempC = &t
	}

	// Convert humidity
	var humidity *float32
	if condition.Humidity != nil {
		h := float32(*condition.Humidity)
		humidity = &h
	}

	// Assign to fields based on occurrence index
	switch index {
	case 0:
		if tempC != nil {
			reading.OutTemp = *tempC
		}
		if humidity != nil {
			reading.OutHumidity = *humidity
		}
		// Dew point
		if condition.DewPoint != nil {
			// DewPoint is already in Fahrenheit, convert to Celsius
			reading.ExtraFloat1 = float32((*condition.DewPoint - 32) * 5 / 9)
		}
		// Heat index
		if condition.HeatIndex != nil {
			reading.HeatIndex = float32((*condition.HeatIndex - 32) * 5 / 9)
		}
		// Wet bulb
		if condition.WetBulb != nil {
			reading.ExtraFloat2 = float32((*condition.WetBulb - 32) * 5 / 9)
		}

	case 1:
		if tempC != nil {
			reading.ExtraTemp1 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity1 = *humidity
		}

	case 2:
		if tempC != nil {
			reading.ExtraTemp2 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity2 = *humidity
		}

	case 3:
		if tempC != nil {
			reading.ExtraTemp3 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity3 = *humidity
		}

	case 4:
		if tempC != nil {
			reading.ExtraTemp4 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity4 = *humidity
		}

	case 5:
		if tempC != nil {
			reading.ExtraTemp5 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity5 = *humidity
		}

	case 6:
		if tempC != nil {
			reading.ExtraTemp6 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity6 = *humidity
		}

	case 7:
		if tempC != nil {
			reading.ExtraTemp7 = *tempC
		}
		if humidity != nil {
			reading.ExtraHumidity7 = *humidity
		}
	}
}

// applyWind applies wind data
func (s *Station) applyWind(reading *types.Reading, condition *Condition) {
	if condition.WindSpeedLast != nil {
		reading.WindSpeed = float32(*condition.WindSpeedLast)
	}
	if condition.WindDirLast != nil {
		reading.WindDir = float32(*condition.WindDirLast)
	}
}

// applyRain applies rain data
// IMPORTANT: Pass raw values - NO differential calculation
func (s *Station) applyRain(reading *types.Reading, condition *Condition) {
	if condition.RainfallDaily != nil {
		reading.DayRain = float32(*condition.RainfallDaily)
	}
	if condition.RainRateLast != nil {
		reading.RainRate = float32(*condition.RainRateLast)
	}
}

// applySolar applies solar radiation data
func (s *Station) applySolar(reading *types.Reading, condition *Condition) {
	if condition.SolarRad != nil {
		reading.Radiation = float32(*condition.SolarRad)
	}
}

// applyUV applies UV index data
func (s *Station) applyUV(reading *types.Reading, condition *Condition) {
	if condition.UVIndex != nil {
		reading.UV = float32(*condition.UVIndex)
	}
}

// applyWindChill applies wind chill data
func (s *Station) applyWindChill(reading *types.Reading, condition *Condition) {
	if condition.WindChill != nil {
		reading.WindChill = float32((*condition.WindChill - 32) * 5 / 9)
	}
}

// applyTHW applies THW index data
func (s *Station) applyTHW(reading *types.Reading, condition *Condition, options []string) {
	if condition.THWIndex != nil {
		thwC := float32((*condition.THWIndex - 32) * 5 / 9)
		reading.ExtraFloat3 = thwC
		// Check if should map to appTemp (not yet implemented in types.Reading)
		if hasOption(options, "appTemp") {
			reading.ExtraFloat5 = thwC
		}
	}
}

// applyTHSW applies THSW index data
func (s *Station) applyTHSW(reading *types.Reading, condition *Condition, options []string) {
	if condition.THSWIndex != nil {
		thswC := float32((*condition.THSWIndex - 32) * 5 / 9)
		reading.ExtraFloat4 = thswC
		// Check if should map to appTemp
		if hasOption(options, "appTemp") {
			reading.ExtraFloat5 = thswC
		}
	}
}

// applyBarometer applies barometric pressure data
func (s *Station) applyBarometer(reading *types.Reading, condition *Condition) {
	// Convert inHg to hPa
	if condition.BarAbsolute != nil {
		reading.BaromAbsIn = float32(*condition.BarAbsolute * 33.8639)
	}
	if condition.BarSeaLevel != nil {
		reading.BaromRelIn = float32(*condition.BarSeaLevel * 33.8639)
		reading.Barometer = float32(*condition.BarSeaLevel * 33.8639)
	}
}

// applyIndoor applies indoor temperature and humidity
func (s *Station) applyIndoor(reading *types.Reading, condition *Condition) {
	// Convert Fahrenheit to Celsius
	if condition.TempIn != nil {
		reading.InTemp = float32((*condition.TempIn - 32) * 5 / 9)
	}
	if condition.HumIn != nil {
		reading.InHumidity = float32(*condition.HumIn)
	}
	// Indoor dew point
	if condition.DewPointIn != nil {
		reading.ExtraFloat6 = float32((*condition.DewPointIn - 32) * 5 / 9)
	}
	// Indoor heat index
	if condition.HeatIndexIn != nil {
		reading.ExtraFloat7 = float32((*condition.HeatIndexIn - 32) * 5 / 9)
	}
}

// applySoilTemp applies soil temperature data
func (s *Station) applySoilTemp(reading *types.Reading, condition *Condition, port int) {
	var tempF *float64

	// Get temperature from the appropriate field
	switch port {
	case 1:
		tempF = condition.Temp1
	case 2:
		tempF = condition.Temp2
	case 3:
		tempF = condition.Temp3
	case 4:
		tempF = condition.Temp4
	}

	if tempF == nil {
		return
	}

	tempC := float32((*tempF - 32) * 5 / 9)

	// Assign to soil temp field
	switch port {
	case 1:
		reading.SoilTemp1 = tempC
	case 2:
		reading.SoilTemp2 = tempC
	case 3:
		reading.SoilTemp3 = tempC
	case 4:
		reading.SoilTemp4 = tempC
	}
}

// applySoilMoisture applies soil moisture data
func (s *Station) applySoilMoisture(reading *types.Reading, condition *Condition, port int) {
	var moistInt *int

	// Get moisture from the appropriate field
	switch port {
	case 1:
		moistInt = condition.MoistSoil1
	case 2:
		moistInt = condition.MoistSoil2
	case 3:
		moistInt = condition.MoistSoil3
	case 4:
		moistInt = condition.MoistSoil4
	}

	if moistInt == nil {
		return
	}

	moisture := float32(*moistInt)

	// Assign to soil moisture field
	switch port {
	case 1:
		reading.SoilMoisture1 = moisture
	case 2:
		reading.SoilMoisture2 = moisture
	case 3:
		reading.SoilMoisture3 = moisture
	case 4:
		reading.SoilMoisture4 = moisture
	}
}

// applyLeafWetness applies leaf wetness data
func (s *Station) applyLeafWetness(reading *types.Reading, condition *Condition, port int) {
	var wetInt *int

	// Get wetness from the appropriate field
	switch port {
	case 1:
		wetInt = condition.WetLeaf1
	case 2:
		wetInt = condition.WetLeaf2
	}

	if wetInt == nil {
		return
	}

	wetness := float32(*wetInt)

	// Assign to leaf wetness field
	switch port {
	case 1:
		reading.LeafWetness1 = wetness
	case 2:
		reading.LeafWetness2 = wetness
	}
}

// applyBattery applies battery status data
func (s *Station) applyBattery(reading *types.Reading, condition *Condition, txid int) {
	if condition.TransBatteryFlag != nil {
		batteryStatus := uint8(*condition.TransBatteryFlag)

		// Map to battery fields based on txid
		switch txid {
		case 1:
			reading.Batt1 = batteryStatus
		case 2:
			reading.Batt2 = batteryStatus
		case 3:
			reading.Batt3 = batteryStatus
		case 4:
			reading.Batt4 = batteryStatus
		case 5:
			reading.Batt5 = batteryStatus
		case 6:
			reading.Batt6 = batteryStatus
		case 7:
			reading.Batt7 = batteryStatus
		case 8:
			reading.Batt8 = batteryStatus
		}
	}
}

// hasOption checks if an option is in the options list
func hasOption(options []string, option string) bool {
	for _, opt := range options {
		if opt == option {
			return true
		}
	}
	return false
}
