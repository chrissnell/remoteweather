package weatherlinklive

import (
	"fmt"
	"sort"
)

// DiscoveredSensor represents a sensor found during discovery
type DiscoveredSensor struct {
	TxID              *int     `json:"tx_id,omitempty"`
	Type              string   `json:"type"`
	Description       string   `json:"description"`
	Available         bool     `json:"available"`
	DataStructureType int      `json:"data_structure_type,omitempty"`
	Fields            []string `json:"fields,omitempty"`
}

// DiscoveredTransmitter represents a transmitter and its sensors
type DiscoveredTransmitter struct {
	TxID              int                `json:"tx_id"`
	DataStructureType int                `json:"data_structure_type"`
	Sensors           []DiscoveredSensor `json:"sensors"`
	BatteryStatus     *int               `json:"battery_status,omitempty"`
}

// DiscoveryResult contains the discovery results from a WeatherLink Live device
type DiscoveryResult struct {
	DID           string                  `json:"did"`
	Timestamp     int64                   `json:"timestamp"`
	Transmitters  []DiscoveredTransmitter `json:"transmitters"`
	InternalSensors []DiscoveredSensor    `json:"internal_sensors"`
	SuggestedTemplate string                `json:"suggested_template"`
}

// DiscoverSensors analyzes current conditions and returns discovered sensors
func DiscoverSensors(conditions []Condition) *DiscoveryResult {
	result := &DiscoveryResult{
		Transmitters:    make([]DiscoveredTransmitter, 0),
		InternalSensors: make([]DiscoveredSensor, 0),
	}

	// Track transmitters and their sensors
	transmitterMap := make(map[int]*DiscoveredTransmitter)

	// Process each condition
	for _, cond := range conditions {
		// Handle internal sensors (barometer, indoor temp/humidity)
		if cond.TxID == nil {
			result.InternalSensors = append(result.InternalSensors, discoverInternalSensors(cond)...)
			continue
		}

		txid := *cond.TxID

		// Create transmitter entry if it doesn't exist
		if transmitterMap[txid] == nil {
			transmitterMap[txid] = &DiscoveredTransmitter{
				TxID:              txid,
				DataStructureType: cond.DataStructureType,
				Sensors:           make([]DiscoveredSensor, 0),
			}
		}

		tx := transmitterMap[txid]

		// Add battery status
		if cond.TransBatteryFlag != nil {
			tx.BatteryStatus = cond.TransBatteryFlag
		}

		// Discover sensors based on data structure type
		switch cond.DataStructureType {
		case 1:
			// ISS sensor suite (temp/humidity, wind, rain, UV, solar)
			tx.Sensors = append(tx.Sensors, discoverISSSensors(cond)...)
		case 2:
			// Agricultural station (soil temp, soil moisture, leaf wetness)
			tx.Sensors = append(tx.Sensors, discoverAgriculturalSensors(cond)...)
		case 3:
			// Leaf/soil moisture station
			tx.Sensors = append(tx.Sensors, discoverLeafSoilSensors(cond)...)
		case 4:
			// Additional temperature/humidity sensors
			tx.Sensors = append(tx.Sensors, discoverTempHumiditySensors(cond)...)
		}
	}

	// Convert map to sorted slice
	txids := make([]int, 0, len(transmitterMap))
	for txid := range transmitterMap {
		txids = append(txids, txid)
	}
	sort.Ints(txids)

	for _, txid := range txids {
		result.Transmitters = append(result.Transmitters, *transmitterMap[txid])
	}

	// Suggest template based on discovered sensors
	result.SuggestedTemplate = SuggestTemplate(conditions)

	return result
}

// discoverInternalSensors discovers internal sensors (barometer, indoor temp/humidity)
func discoverInternalSensors(cond Condition) []DiscoveredSensor {
	sensors := make([]DiscoveredSensor, 0)

	// Barometer
	if cond.BarAbsolute != nil || cond.BarSeaLevel != nil {
		fields := make([]string, 0)
		if cond.BarAbsolute != nil {
			fields = append(fields, "bar_absolute")
		}
		if cond.BarSeaLevel != nil {
			fields = append(fields, "bar_sea_level")
		}

		sensors = append(sensors, DiscoveredSensor{
			Type:        "baro",
			Description: "Barometric Pressure",
			Available:   true,
			Fields:      fields,
		})
	}

	// Indoor temperature/humidity
	if cond.TempIn != nil || cond.HumIn != nil {
		fields := make([]string, 0)
		if cond.TempIn != nil {
			fields = append(fields, "temp_in")
		}
		if cond.HumIn != nil {
			fields = append(fields, "hum_in")
		}
		if cond.DewPointIn != nil {
			fields = append(fields, "dew_point_in")
		}
		if cond.HeatIndexIn != nil {
			fields = append(fields, "heat_index_in")
		}

		sensors = append(sensors, DiscoveredSensor{
			Type:        "th_indoor",
			Description: "Indoor Temperature/Humidity",
			Available:   true,
			Fields:      fields,
		})
	}

	return sensors
}

// discoverISSSensors discovers ISS (Integrated Sensor Suite) sensors
func discoverISSSensors(cond Condition) []DiscoveredSensor {
	sensors := make([]DiscoveredSensor, 0)
	txid := cond.TxID

	// Temperature/Humidity
	if cond.Temp != nil || cond.Humidity != nil {
		fields := make([]string, 0)
		if cond.Temp != nil {
			fields = append(fields, "temp")
		}
		if cond.Humidity != nil {
			fields = append(fields, "hum")
		}
		if cond.DewPoint != nil {
			fields = append(fields, "dew_point")
		}
		if cond.WetBulb != nil {
			fields = append(fields, "wet_bulb")
		}
		if cond.HeatIndex != nil {
			fields = append(fields, "heat_index")
		}

		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "th",
			Description:       fmt.Sprintf("Temperature/Humidity (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            fields,
		})
	}

	// Wind
	if cond.WindSpeedLast != nil || cond.WindDirLast != nil {
		fields := make([]string, 0)
		if cond.WindSpeedLast != nil {
			fields = append(fields, "wind_speed_last")
		}
		if cond.WindDirLast != nil {
			fields = append(fields, "wind_dir_last")
		}

		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "wind",
			Description:       fmt.Sprintf("Wind Speed/Direction (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            fields,
		})
	}

	// Rain
	if cond.RainfallDaily != nil || cond.RainRateLast != nil {
		fields := make([]string, 0)
		if cond.RainfallDaily != nil {
			fields = append(fields, "rainfall_daily")
		}
		if cond.RainRateLast != nil {
			fields = append(fields, "rain_rate_last")
		}
		if cond.RainSize != nil {
			fields = append(fields, "rain_size")
		}

		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "rain",
			Description:       fmt.Sprintf("Rain (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            fields,
		})
	}

	// Solar Radiation
	if cond.SolarRad != nil {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "solar",
			Description:       fmt.Sprintf("Solar Radiation (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            []string{"solar_rad"},
		})
	}

	// UV Index
	if cond.UVIndex != nil {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "uv",
			Description:       fmt.Sprintf("UV Index (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            []string{"uv_index"},
		})
	}

	// Wind Chill (compound index)
	if cond.WindChill != nil {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "windchill",
			Description:       fmt.Sprintf("Wind Chill (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            []string{"wind_chill"},
		})
	}

	// THW Index (Temperature-Humidity-Wind)
	if cond.THWIndex != nil {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "thw",
			Description:       fmt.Sprintf("THW Index (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            []string{"thw_index"},
		})
	}

	// THSW Index (Temperature-Humidity-Solar-Wind)
	if cond.THSWIndex != nil {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "thsw",
			Description:       fmt.Sprintf("THSW Index (TX%d)", *txid),
			Available:         true,
			DataStructureType: 1,
			Fields:            []string{"thsw_index"},
		})
	}

	return sensors
}

// discoverAgriculturalSensors discovers agricultural station sensors
func discoverAgriculturalSensors(cond Condition) []DiscoveredSensor {
	sensors := make([]DiscoveredSensor, 0)
	txid := cond.TxID

	// Soil Temperature sensors (up to 4)
	soilTempPorts := make([]int, 0)
	if cond.Temp1 != nil {
		soilTempPorts = append(soilTempPorts, 1)
	}
	if cond.Temp2 != nil {
		soilTempPorts = append(soilTempPorts, 2)
	}
	if cond.Temp3 != nil {
		soilTempPorts = append(soilTempPorts, 3)
	}
	if cond.Temp4 != nil {
		soilTempPorts = append(soilTempPorts, 4)
	}

	for _, port := range soilTempPorts {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "soil_temp",
			Description:       fmt.Sprintf("Soil Temperature Port %d (TX%d)", port, *txid),
			Available:         true,
			DataStructureType: 2,
			Fields:            []string{fmt.Sprintf("temp_%d", port)},
		})
	}

	// Soil Moisture sensors (up to 4)
	soilMoistPorts := make([]int, 0)
	if cond.MoistSoil1 != nil {
		soilMoistPorts = append(soilMoistPorts, 1)
	}
	if cond.MoistSoil2 != nil {
		soilMoistPorts = append(soilMoistPorts, 2)
	}
	if cond.MoistSoil3 != nil {
		soilMoistPorts = append(soilMoistPorts, 3)
	}
	if cond.MoistSoil4 != nil {
		soilMoistPorts = append(soilMoistPorts, 4)
	}

	for _, port := range soilMoistPorts {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "soil_moist",
			Description:       fmt.Sprintf("Soil Moisture Port %d (TX%d)", port, *txid),
			Available:         true,
			DataStructureType: 2,
			Fields:            []string{fmt.Sprintf("moist_soil_%d", port)},
		})
	}

	// Leaf Wetness sensors (up to 2)
	leafWetPorts := make([]int, 0)
	if cond.WetLeaf1 != nil {
		leafWetPorts = append(leafWetPorts, 1)
	}
	if cond.WetLeaf2 != nil {
		leafWetPorts = append(leafWetPorts, 2)
	}

	for _, port := range leafWetPorts {
		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "leaf_wet",
			Description:       fmt.Sprintf("Leaf Wetness Port %d (TX%d)", port, *txid),
			Available:         true,
			DataStructureType: 2,
			Fields:            []string{fmt.Sprintf("wet_leaf_%d", port)},
		})
	}

	return sensors
}

// discoverLeafSoilSensors discovers leaf/soil moisture station sensors
func discoverLeafSoilSensors(cond Condition) []DiscoveredSensor {
	// Data structure type 3 - similar to agricultural but different structure
	return discoverAgriculturalSensors(cond)
}

// discoverTempHumiditySensors discovers additional temperature/humidity sensors
func discoverTempHumiditySensors(cond Condition) []DiscoveredSensor {
	sensors := make([]DiscoveredSensor, 0)
	txid := cond.TxID

	// Additional temp/humidity sensor
	if cond.Temp != nil || cond.Humidity != nil {
		fields := make([]string, 0)
		if cond.Temp != nil {
			fields = append(fields, "temp")
		}
		if cond.Humidity != nil {
			fields = append(fields, "hum")
		}
		if cond.DewPoint != nil {
			fields = append(fields, "dew_point")
		}

		sensors = append(sensors, DiscoveredSensor{
			TxID:              txid,
			Type:              "th",
			Description:       fmt.Sprintf("Temperature/Humidity (TX%d)", *txid),
			Available:         true,
			DataStructureType: 4,
			Fields:            fields,
		})
	}

	return sensors
}
