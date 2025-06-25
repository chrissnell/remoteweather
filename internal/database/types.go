package database

import "time"

// FetchedBucketReading represents a reading fetched from the database with aggregated data
type FetchedBucketReading struct {
	Bucket                *time.Time `gorm:"column:bucket"`
	StationName           string     `gorm:"column:stationname"`
	Barometer             float32    `gorm:"column:barometer"`
	MaxBarometer          float32    `gorm:"column:max_barometer"`
	MinBarometer          float32    `gorm:"column:min_barometer"`
	InTemp                float32    `gorm:"column:intemp"`
	MaxInTemp             float32    `gorm:"column:max_intemp"`
	MinInTemp             float32    `gorm:"column:max_intemp"`
	ExtraTemp1            float32    `gorm:"column:extratemp1"`
	MaxExtraTemp1         float32    `gorm:"column:min_extratemp1"`
	MinExtraTemp1         float32    `gorm:"column:max_extratemp1"`
	InHumidity            float32    `gorm:"column:inhumidity"`
	MaxInHumidity         float32    `gorm:"column:max_inhumidity"`
	MinInHumidity         float32    `gorm:"column:min_inhumidity"`
	SolarWatts            float32    `gorm:"column:solarwatts"`
	PotentialSolarWatts   float32    `gorm:"column:potentialsolarwatts"`
	SolarJoules           float32    `gorm:"column:solarjoules"`
	OutTemp               float32    `gorm:"column:outtemp"`
	MaxOutTemp            float32    `gorm:"column:max_outtemp"`
	MinOutTemp            float32    `gorm:"column:min_outtemp"`
	OutHumidity           float32    `gorm:"column:outhumidity"`
	MinOutHumidity        float32    `gorm:"column:min_outhumidity"`
	MaxOutHumidity        float32    `gorm:"column:max_outhumidity"`
	WindSpeed             float32    `gorm:"column:windspeed"`
	MaxWindSpeed          float32    `gorm:"column:max_windspeed"`
	WindDir               float32    `gorm:"column:winddir"`
	WindChill             float32    `gorm:"column:windchill"`
	MinWindChill          float32    `gorm:"column:min_windchill"`
	HeatIndex             float32    `gorm:"column:heatindex"`
	MaxHeatIndex          float32    `gorm:"column:max_heatindex"`
	PeriodRain            float32    `gorm:"column:period_rain"`
	RainRate              float32    `gorm:"column:rainrate"`
	MaxRainRate           float32    `gorm:"column:max_rainrate"`
	DayRain               float32    `gorm:"column:dayrain"`
	MonthRain             float32    `gorm:"column:monthrain"`
	YearRain              float32    `gorm:"column:yearrain"`
	ConsBatteryVoltage    float32    `gorm:"column:consbatteryvoltage"`
	StationBatteryVoltage float32    `gorm:"column:stationbatteryvoltage"`
}

// TableName implements the Tabler interface for the FetchedBucketReading struct
func (FetchedBucketReading) TableName() string {
	return "weather_1m"
}
