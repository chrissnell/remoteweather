// Package types defines weather-related data structures and types.
package types

import (
	"reflect"
	"time"
)

// Reading is a generic weather reading, containing human-readable values
// for most commonly-reported weather metrics.  When creating a new WeatherStation
// implementation, you should ideally use one of the existing Reading struct members.
// If you can't find what you need in here, you can add a new member to the struct.
type Reading struct {
	Timestamp             time.Time `gorm:"column:time"`
	StationName           string    `gorm:"column:stationname"`
	StationType           string    `gorm:"column:stationtype"`
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
	// Additional temperature sensors (temp1f-10f)
	Temp1                 float32   `gorm:"column:temp1"`
	Temp2                 float32   `gorm:"column:temp2"`
	Temp3                 float32   `gorm:"column:temp3"`
	Temp4                 float32   `gorm:"column:temp4"`
	Temp5                 float32   `gorm:"column:temp5"`
	Temp6                 float32   `gorm:"column:temp6"`
	Temp7                 float32   `gorm:"column:temp7"`
	Temp8                 float32   `gorm:"column:temp8"`
	Temp9                 float32   `gorm:"column:temp9"`
	Temp10                float32   `gorm:"column:temp10"`
	SoilTemp1             float32   `gorm:"column:soiltemp1"`
	SoilTemp2             float32   `gorm:"column:soiltemp2"`
	SoilTemp3             float32   `gorm:"column:soiltemp3"`
	SoilTemp4             float32   `gorm:"column:soiltemp4"`
	// Additional soil temperature sensors (soiltemp5f-10f)
	SoilTemp5             float32   `gorm:"column:soiltemp5"`
	SoilTemp6             float32   `gorm:"column:soiltemp6"`
	SoilTemp7             float32   `gorm:"column:soiltemp7"`
	SoilTemp8             float32   `gorm:"column:soiltemp8"`
	SoilTemp9             float32   `gorm:"column:soiltemp9"`
	SoilTemp10            float32   `gorm:"column:soiltemp10"`
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
	// Additional humidity sensors (humidity1-10)
	Humidity1             float32   `gorm:"column:humidity1"`
	Humidity2             float32   `gorm:"column:humidity2"`
	Humidity3             float32   `gorm:"column:humidity3"`
	Humidity4             float32   `gorm:"column:humidity4"`
	Humidity5             float32   `gorm:"column:humidity5"`
	Humidity6             float32   `gorm:"column:humidity6"`
	Humidity7             float32   `gorm:"column:humidity7"`
	Humidity8             float32   `gorm:"column:humidity8"`
	Humidity9             float32   `gorm:"column:humidity9"`
	Humidity10            float32   `gorm:"column:humidity10"`
	// Soil humidity sensors (soilhum1-10)
	SoilHum1              float32   `gorm:"column:soilhum1"`
	SoilHum2              float32   `gorm:"column:soilhum2"`
	SoilHum3              float32   `gorm:"column:soilhum3"`
	SoilHum4              float32   `gorm:"column:soilhum4"`
	SoilHum5              float32   `gorm:"column:soilhum5"`
	SoilHum6              float32   `gorm:"column:soilhum6"`
	SoilHum7              float32   `gorm:"column:soilhum7"`
	SoilHum8              float32   `gorm:"column:soilhum8"`
	SoilHum9              float32   `gorm:"column:soilhum9"`
	SoilHum10             float32   `gorm:"column:soilhum10"`
	LeafWetness1          float32   `gorm:"column:leafwetness1"`
	LeafWetness2          float32   `gorm:"column:leafwetness2"`
	LeafWetness3          float32   `gorm:"column:leafwetness3"`
	LeafWetness4          float32   `gorm:"column:leafwetness4"`
	// Additional leaf wetness sensors (leafwetness5-8)
	LeafWetness5          float32   `gorm:"column:leafwetness5"`
	LeafWetness6          float32   `gorm:"column:leafwetness6"`
	LeafWetness7          float32   `gorm:"column:leafwetness7"`
	LeafWetness8          float32   `gorm:"column:leafwetness8"`
	// Soil tension sensors (soiltens1-4)
	SoilTens1             float32   `gorm:"column:soiltens1"`
	SoilTens2             float32   `gorm:"column:soiltens2"`
	SoilTens3             float32   `gorm:"column:soiltens3"`
	SoilTens4             float32   `gorm:"column:soiltens4"`
	RainRate              float32   `gorm:"column:rainrate"`
	RainIncremental       float32   `gorm:"column:rainincremental"`
	SolarWatts            float32   `gorm:"column:solarwatts"`
	PotentialSolarWatts   float32   `gorm:"column:potentialsolarwatts"`
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
	// Growing degree days and evapotranspiration
	GDD                   int32     `gorm:"column:gdd"`
	ETOS                  float32   `gorm:"column:etos"`
	ETRS                  float32   `gorm:"column:etrs"`
	SoilMoisture1         float32   `gorm:"column:soilmoisture1"`
	SoilMoisture2         float32   `gorm:"column:soilmoisture2"`
	SoilMoisture3         float32   `gorm:"column:soilmoisture3"`
	SoilMoisture4         float32   `gorm:"column:soilmoisture4"`
	// Leak detection sensors (leak1-4)
	Leak1                 uint8     `gorm:"column:leak1"`
	Leak2                 uint8     `gorm:"column:leak2"`
	Leak3                 uint8     `gorm:"column:leak3"`
	Leak4                 uint8     `gorm:"column:leak4"`
	// Relay states (relay1-10)
	Relay1                uint8     `gorm:"column:relay1"`
	Relay2                uint8     `gorm:"column:relay2"`
	Relay3                uint8     `gorm:"column:relay3"`
	Relay4                uint8     `gorm:"column:relay4"`
	Relay5                uint8     `gorm:"column:relay5"`
	Relay6                uint8     `gorm:"column:relay6"`
	Relay7                uint8     `gorm:"column:relay7"`
	Relay8                uint8     `gorm:"column:relay8"`
	Relay9                uint8     `gorm:"column:relay9"`
	Relay10               uint8     `gorm:"column:relay10"`
	// Air quality measurements
	PM25                  float32   `gorm:"column:pm25"`
	PM25_24H              float32   `gorm:"column:pm25_24h"`
	PM25In                float32   `gorm:"column:pm25_in"`
	PM25In24H             float32   `gorm:"column:pm25_in_24h"`
	PM25InAQIN            float32   `gorm:"column:pm25_in_aqin"`
	PM25In24HAQIN         float32   `gorm:"column:pm25_in_24h_aqin"`
	PM10InAQIN            float32   `gorm:"column:pm10_in_aqin"`
	PM10In24HAQIN         float32   `gorm:"column:pm10_in_24h_aqin"`
	CO2                   float32   `gorm:"column:co2"`
	CO2InAQIN             int32     `gorm:"column:co2_in_aqin"`
	CO2In24HAQIN          int32     `gorm:"column:co2_in_24h_aqin"`
	PMInTempAQIN          float32   `gorm:"column:pm_in_temp_aqin"`
	PMInHumidityAQIN      int32     `gorm:"column:pm_in_humidity_aqin"`
	AQIPM25AQIN           int32     `gorm:"column:aqi_pm25_aqin"`
	AQIPM2524HAQIN        int32     `gorm:"column:aqi_pm25_24h_aqin"`
	AQIPM10AQIN           int32     `gorm:"column:aqi_pm10_aqin"`
	AQIPM1024HAQIN        int32     `gorm:"column:aqi_pm10_24h_aqin"`
	AQIPM25In             int32     `gorm:"column:aqi_pm25_in"`
	AQIPM25In24H          int32     `gorm:"column:aqi_pm25_in_24h"`
	// Lightning data
	LightningDay          int32     `gorm:"column:lightning_day"`
	LightningHour         int32     `gorm:"column:lightning_hour"`
	LightningTime         time.Time `gorm:"column:lightning_time"`
	LightningDistance     float32   `gorm:"column:lightning_distance"`
	// Pressure measurements
	BaromRelIn            float32   `gorm:"column:baromrelin"`
	BaromAbsIn            float32   `gorm:"column:baromabsin"`
	// Battery status
	BattOut               uint8     `gorm:"column:battout"`
	BattIn                uint8     `gorm:"column:battin"`
	Batt1                 uint8     `gorm:"column:batt1"`
	Batt2                 uint8     `gorm:"column:batt2"`
	Batt3                 uint8     `gorm:"column:batt3"`
	Batt4                 uint8     `gorm:"column:batt4"`
	Batt5                 uint8     `gorm:"column:batt5"`
	Batt6                 uint8     `gorm:"column:batt6"`
	Batt7                 uint8     `gorm:"column:batt7"`
	Batt8                 uint8     `gorm:"column:batt8"`
	Batt9                 uint8     `gorm:"column:batt9"`
	Batt10                uint8     `gorm:"column:batt10"`
	Batt25                uint8     `gorm:"column:batt_25"`
	BattLightning         uint8     `gorm:"column:batt_lightning"`
	BatLeak1              uint8     `gorm:"column:batleak1"`
	BatLeak2              uint8     `gorm:"column:batleak2"`
	BatLeak3              uint8     `gorm:"column:batleak3"`
	BatLeak4              uint8     `gorm:"column:batleak4"`
	BattSM1               uint8     `gorm:"column:battsm1"`
	BattSM2               uint8     `gorm:"column:battsm2"`
	BattSM3               uint8     `gorm:"column:battsm3"`
	BattSM4               uint8     `gorm:"column:battsm4"`
	BattCO2               uint8     `gorm:"column:batt_co2"`
	BattCellGateway       uint8     `gorm:"column:batt_cellgateway"`
	// Time zone and UTC time
	TZ                    string    `gorm:"column:tz"`
	DateUTC               int64     `gorm:"column:dateutc"`
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
	SnowDistance          float32   `gorm:"column:snowdistance"`
	SnowDepth             float32   `gorm:"column:snowdepth"`
	SnowDepthEst          float32   `gorm:"-"` // Estimated/smoothed depth (not a DB column)
	ExtraFloat1           float32   `gorm:"column:extrafloat1"`
	ExtraFloat2           float32   `gorm:"column:extrafloat2"`
	ExtraFloat3           float32   `gorm:"column:extrafloat3"`
	ExtraFloat4           float32   `gorm:"column:extrafloat4"`
	ExtraFloat5           float32   `gorm:"column:extrafloat5"`
	ExtraFloat6           float32   `gorm:"column:extrafloat6"`
	ExtraFloat7           float32   `gorm:"column:extrafloat7"`
	ExtraFloat8           float32   `gorm:"column:extrafloat8"`
	ExtraFloat9           float32   `gorm:"column:extrafloat9"`
	ExtraFloat10          float32   `gorm:"column:extrafloat10"`
	ExtraText1            string    `gorm:"column:extratext1"`
	ExtraText2            string    `gorm:"column:extratext2"`
	ExtraText3            string    `gorm:"column:extratext3"`
	ExtraText4            string    `gorm:"column:extratext4"`
	ExtraText5            string    `gorm:"column:extratext5"`
	ExtraText6            string    `gorm:"column:extratext6"`
	ExtraText7            string    `gorm:"column:extratext7"`
	ExtraText8            string    `gorm:"column:extratext8"`
	ExtraText9            string    `gorm:"column:extratext9"`
	ExtraText10           string    `gorm:"column:extratext10"`
}

// ToMap converts a Reading object into a map for later storage
func (r *Reading) ToMap() map[string]interface{} {
	m := make(map[string]interface{})

	v := reflect.ValueOf(*r)

	for i := 0; i < v.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Float32:
			m[v.Type().Field(i).Name] = v.Field(i).Float()
		case reflect.Uint8:
			m[v.Type().Field(i).Name] = v.Field(i).Uint()
		}
	}

	return m
}

// TableName implements the GORM Tabler interface for the Reading struct
func (Reading) TableName() string {
	return "weather"
}

// BucketReading is a Reading with a few extra fields that are present in the materialized view
type BucketReading struct {
	Bucket     time.Time `gorm:"column:bucket"`
	PeriodRain float32   `gorm:"column:period_rain"`
	Reading
}
