// Package weatherlinklive provides support for Davis Instruments WeatherLink Live devices
package weatherlinklive

// Data structure types from WeatherLink Live API
const (
	DataStructureISS     = 1 // Integrated Sensor Suite
	DataStructureLeafSoil = 2 // Leaf/soil moisture sensors
	DataStructureBarometer = 3 // WLL internal barometer
	DataStructureTempHum  = 4 // WLL internal temp/humidity
)

// APIError represents an error response from the WLL API
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CurrentConditionsResponse is the response from /v1/current_conditions
type CurrentConditionsResponse struct {
	Data  CurrentConditionsData `json:"data"`
	Error *APIError             `json:"error"`
}

// CurrentConditionsData contains the actual weather data
type CurrentConditionsData struct {
	DID        string      `json:"did"`
	Timestamp  int64       `json:"ts"`
	Conditions []Condition `json:"conditions"`
}

// Condition represents a single sensor reading
// All fields are pointers because they can be null in the JSON response
type Condition struct {
	DataStructureType int      `json:"data_structure_type"`
	TxID              *int     `json:"txid"`

	// ISS (DataStructureISS) fields
	Temp              *float64 `json:"temp"`
	Humidity          *int     `json:"hum"`
	DewPoint          *float64 `json:"dew_point"`
	WetBulb           *float64 `json:"wet_bulb"`
	HeatIndex         *float64 `json:"heat_index"`
	WindChill         *float64 `json:"wind_chill"`
	THWIndex          *float64 `json:"thw_index"`
	THSWIndex         *float64 `json:"thsw_index"`
	WindSpeedLast     *float64 `json:"wind_speed_last"`
	WindDirLast       *int     `json:"wind_dir_last"`
	RainSize          *int     `json:"rain_size"`
	RainRateLast      *float64 `json:"rain_rate_last"`
	RainfallDaily     *float64 `json:"rainfall_daily"`
	SolarRad          *int     `json:"solar_rad"`
	UVIndex           *float64 `json:"uv_index"`
	TransBatteryFlag  *int     `json:"trans_battery_flag"`

	// Barometer (DataStructureBarometer) fields
	BarAbsolute       *float64 `json:"bar_absolute"`
	BarSeaLevel       *float64 `json:"bar_sea_level"`

	// Internal temp/humidity (DataStructureTempHum) fields
	TempIn            *float64 `json:"temp_in"`
	HumIn             *int     `json:"hum_in"`
	DewPointIn        *float64 `json:"dew_point_in"`
	HeatIndexIn       *float64 `json:"heat_index_in"`

	// Agricultural (DataStructureLeafSoil) fields
	Temp1             *float64 `json:"temp_1"`
	Temp2             *float64 `json:"temp_2"`
	Temp3             *float64 `json:"temp_3"`
	Temp4             *float64 `json:"temp_4"`
	MoistSoil1        *int     `json:"moist_soil_1"`
	MoistSoil2        *int     `json:"moist_soil_2"`
	MoistSoil3        *int     `json:"moist_soil_3"`
	MoistSoil4        *int     `json:"moist_soil_4"`
	WetLeaf1          *int     `json:"wet_leaf_1"`
	WetLeaf2          *int     `json:"wet_leaf_2"`
}

// RealTimeResponse is the response from /v1/real_time
type RealTimeResponse struct {
	Data  RealTimeData `json:"data"`
	Error *APIError    `json:"error"`
}

// RealTimeData contains UDP broadcast configuration
type RealTimeData struct {
	BroadcastPort int `json:"broadcast_port"`
	Duration      int `json:"duration"`
}

// SensorMapping defines how to map a sensor to reading fields
type SensorMapping struct {
	Type    string   // "th", "wind", "rain", etc.
	TxID    *int     // Transmitter ID (nil for internal sensors)
	Port    *int     // For soil/leaf sensors (1-4)
	Options []string // e.g., ["appTemp"]
}
