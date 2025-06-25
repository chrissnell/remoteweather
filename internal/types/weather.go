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
	SoilTemp1             float32   `gorm:"column:soiltemp1"`
	SoilTemp2             float32   `gorm:"column:soiltemp2"`
	SoilTemp3             float32   `gorm:"column:soiltemp3"`
	SoilTemp4             float32   `gorm:"column:soiltemp4"`
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
	SoilMoisture1         float32   `gorm:"column:soilmoisture1"`
	SoilMoisture2         float32   `gorm:"column:soilmoisture2"`
	SoilMoisture3         float32   `gorm:"column:soilmoisture3"`
	SoilMoisture4         float32   `gorm:"column:soilmoisture4"`
	LeafWetness1          float32   `gorm:"column:leafwetness1"`
	LeafWetness2          float32   `gorm:"column:leafwetness2"`
	LeafWetness3          float32   `gorm:"column:leafwetness3"`
	LeafWetness4          float32   `gorm:"column:leafwetness4"`
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
