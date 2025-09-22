// Package ambientcustomized provides Ambient Weather station support with HTTP customized protocol.
package ambientcustomized

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

type Station struct {
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 *sync.WaitGroup
	server             *http.Server
	config             config.DeviceData
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
	previousTotalRain  float32
	mu                 sync.Mutex
}

func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	if deviceConfig.Port == "" {
		logger.Fatalf("Ambient Weather station [%s] must define a port", deviceConfig.Name)
	}

	// Create a cancellable context for this specific station
	stationCtx, cancel := context.WithCancel(ctx)

	return &Station{
		ctx:                stationCtx,
		cancel:             cancel,
		wg:                 wg,
		config:             *deviceConfig,
		ReadingDistributor: distributor,
		logger:             logger,
	}
}

func (s *Station) StationName() string {
	return s.config.Name
}

func (s *Station) StartWeatherStation() error {
	// Use configured path or default to "/"
	path := s.config.Path
	if path == "" {
		path = "/"
	}
	s.logger.Infof("Starting Ambient Weather customized server station [%s] on port %s, path %s", s.config.Name, s.config.Port, path)

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handleWeatherUpdate)

	listenAddr := "0.0.0.0"
	if s.config.Hostname != "" {
		listenAddr = s.config.Hostname
	}

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", listenAddr, s.config.Port),
		Handler: mux,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("Ambient Weather station [%s] HTTP server error: %v", s.config.Name, err)
		}
	}()

	go func() {
		<-s.ctx.Done()
		s.logger.Infof("Shutting down Ambient Weather station [%s] HTTP server", s.config.Name)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Errorf("Ambient Weather station [%s] HTTP server shutdown error: %v", s.config.Name, err)
		}
	}()

	return nil
}

func (s *Station) StopWeatherStation() error {
	s.logger.Infof("Stopping Ambient Weather station [%s]", s.config.Name)
	s.cancel()
	return nil
}

func (s *Station) handleWeatherUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()

	stationID := query.Get("ID")
	password := query.Get("PASSWORD")
	dateUTC := query.Get("dateutc")
	action := query.Get("action")

	if stationID == "" || password == "" || dateUTC == "" || action != "updateraw" {
		s.logger.Debugf("Ambient Weather station [%s] received invalid request: missing required parameters", s.config.Name)
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", dateUTC)
	if err != nil {
		s.logger.Debugf("Ambient Weather station [%s] failed to parse timestamp '%s': %v", s.config.Name, dateUTC, err)
		http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
		return
	}

	timestamp = timestamp.UTC()

	reading := types.Reading{
		Timestamp:   timestamp,
		StationName: s.config.Name,
		StationType: "ambient-customized",
	}

	s.parseWeatherData(query, &reading)

	reading.WindChill = weatherstations.CalculateWindChill(reading.OutTemp, reading.WindSpeed)
	reading.HeatIndex = weatherstations.CalculateHeatIndex(reading.OutTemp, reading.OutHumidity)

	s.logger.Debugf("Ambient Weather station [%s] received update: temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph @ %.0f°, pressure=%.2f\"",
		s.config.Name, reading.OutTemp, reading.OutHumidity, reading.WindSpeed, reading.WindDir, reading.Barometer)

	s.ReadingDistributor <- reading

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}

func (s *Station) parseWeatherData(query map[string][]string, reading *types.Reading) {
	parseFloat := func(key string, target *float32) {
		if vals, ok := query[key]; ok && len(vals) > 0 && vals[0] != "" {
			if parsed, err := strconv.ParseFloat(vals[0], 32); err == nil {
				*target = float32(parsed)
			}
		}
	}

	parseInt := func(key string, target *int32) {
		if vals, ok := query[key]; ok && len(vals) > 0 && vals[0] != "" {
			if parsed, err := strconv.ParseInt(vals[0], 10, 32); err == nil {
				*target = int32(parsed)
			}
		}
	}

	parseUint8 := func(key string, target *uint8) {
		if vals, ok := query[key]; ok && len(vals) > 0 && vals[0] != "" {
			if parsed, err := strconv.ParseUint(vals[0], 10, 8); err == nil {
				*target = uint8(parsed)
			}
		}
	}

	parseInt64 := func(key string, target *int64) {
		if vals, ok := query[key]; ok && len(vals) > 0 && vals[0] != "" {
			if parsed, err := strconv.ParseInt(vals[0], 10, 64); err == nil {
				*target = parsed
			}
		}
	}

	parseString := func(key string, target *string) {
		if vals, ok := query[key]; ok && len(vals) > 0 {
			*target = vals[0]
		}
	}

	parseTime := func(key string, target *time.Time) {
		if vals, ok := query[key]; ok && len(vals) > 0 && vals[0] != "" {
			if parsed, err := time.Parse(time.RFC3339, vals[0]); err == nil {
				*target = parsed
			}
		}
	}

	// Original fields
	parseFloat("tempf", &reading.OutTemp)
	parseFloat("humidity", &reading.OutHumidity)
	parseFloat("dewptf", &reading.ExtraTemp1)
	parseFloat("winddir", &reading.WindDir)
	// Apply wind direction correction
	reading.WindDir = s.correctWindDirection(reading.WindDir)
	parseFloat("windspeedmph", &reading.WindSpeed)
	parseFloat("windgustmph", &reading.WindSpeed10)
	parseFloat("rainin", &reading.RainIncremental)
	parseFloat("dailyrainin", &reading.DayRain)
	parseFloat("weeklyrainin", &reading.ExtraFloat1)
	parseFloat("monthlyrainin", &reading.MonthRain)
	parseFloat("solarradiation", &reading.SolarWatts)
	parseFloat("UV", &reading.UV)
	parseFloat("indoortempf", &reading.InTemp)
	parseFloat("indoorhumidity", &reading.InHumidity)
	parseFloat("baromin", &reading.Barometer)

	// Handle totalrainin with incremental calculation
	s.mu.Lock()
	var totalRain float32
	if vals, ok := query["totalrainin"]; ok && len(vals) > 0 && vals[0] != "" {
		if parsed, err := strconv.ParseFloat(vals[0], 32); err == nil {
			totalRain = float32(parsed)
			if s.previousTotalRain > 0 && totalRain >= s.previousTotalRain {
				reading.RainIncremental = totalRain - s.previousTotalRain
			} else {
				// Station was reset or first reading
				reading.RainIncremental = 0
			}
			s.previousTotalRain = totalRain
		}
	}
	s.mu.Unlock()

	// Temperature sensors 1-10
	parseFloat("temp1f", &reading.Temp1)
	parseFloat("temp2f", &reading.Temp2)
	parseFloat("temp3f", &reading.Temp3)
	parseFloat("temp4f", &reading.Temp4)
	parseFloat("temp5f", &reading.Temp5)
	parseFloat("temp6f", &reading.Temp6)
	parseFloat("temp7f", &reading.Temp7)
	parseFloat("temp8f", &reading.Temp8)
	parseFloat("temp9f", &reading.Temp9)
	parseFloat("temp10f", &reading.Temp10)

	// Soil temperature sensors 1-10
	parseFloat("soiltemp1f", &reading.SoilTemp1)
	parseFloat("soiltemp2f", &reading.SoilTemp2)
	parseFloat("soiltemp3f", &reading.SoilTemp3)
	parseFloat("soiltemp4f", &reading.SoilTemp4)
	parseFloat("soiltemp5f", &reading.SoilTemp5)
	parseFloat("soiltemp6f", &reading.SoilTemp6)
	parseFloat("soiltemp7f", &reading.SoilTemp7)
	parseFloat("soiltemp8f", &reading.SoilTemp8)
	parseFloat("soiltemp9f", &reading.SoilTemp9)
	parseFloat("soiltemp10f", &reading.SoilTemp10)

	// Humidity sensors 1-10
	parseFloat("humidity1", &reading.Humidity1)
	parseFloat("humidity2", &reading.Humidity2)
	parseFloat("humidity3", &reading.Humidity3)
	parseFloat("humidity4", &reading.Humidity4)
	parseFloat("humidity5", &reading.Humidity5)
	parseFloat("humidity6", &reading.Humidity6)
	parseFloat("humidity7", &reading.Humidity7)
	parseFloat("humidity8", &reading.Humidity8)
	parseFloat("humidity9", &reading.Humidity9)
	parseFloat("humidity10", &reading.Humidity10)

	// Soil humidity sensors 1-10
	parseFloat("soilhum1", &reading.SoilHum1)
	parseFloat("soilhum2", &reading.SoilHum2)
	parseFloat("soilhum3", &reading.SoilHum3)
	parseFloat("soilhum4", &reading.SoilHum4)
	parseFloat("soilhum5", &reading.SoilHum5)
	parseFloat("soilhum6", &reading.SoilHum6)
	parseFloat("soilhum7", &reading.SoilHum7)
	parseFloat("soilhum8", &reading.SoilHum8)
	parseFloat("soilhum9", &reading.SoilHum9)
	parseFloat("soilhum10", &reading.SoilHum10)

	// Leaf wetness sensors 1-8
	parseFloat("leafwetness1", &reading.LeafWetness1)
	parseFloat("leafwetness2", &reading.LeafWetness2)
	parseFloat("leafwetness3", &reading.LeafWetness3)
	parseFloat("leafwetness4", &reading.LeafWetness4)
	parseFloat("leafwetness5", &reading.LeafWetness5)
	parseFloat("leafwetness6", &reading.LeafWetness6)
	parseFloat("leafwetness7", &reading.LeafWetness7)
	parseFloat("leafwetness8", &reading.LeafWetness8)

	// Soil tension sensors 1-4
	parseFloat("soiltens1", &reading.SoilTens1)
	parseFloat("soiltens2", &reading.SoilTens2)
	parseFloat("soiltens3", &reading.SoilTens3)
	parseFloat("soiltens4", &reading.SoilTens4)

	// Agricultural measurements
	parseInt("gdd", &reading.GDD)
	parseFloat("etos", &reading.ETOS)
	parseFloat("etrs", &reading.ETRS)

	// Leak detection sensors 1-4
	parseUint8("leak1", &reading.Leak1)
	parseUint8("leak2", &reading.Leak2)
	parseUint8("leak3", &reading.Leak3)
	parseUint8("leak4", &reading.Leak4)

	// Battery status
	parseUint8("battout", &reading.BattOut)
	parseUint8("battin", &reading.BattIn)
	parseUint8("batt1", &reading.Batt1)
	parseUint8("batt2", &reading.Batt2)
	parseUint8("batt3", &reading.Batt3)
	parseUint8("batt4", &reading.Batt4)
	parseUint8("batt5", &reading.Batt5)
	parseUint8("batt6", &reading.Batt6)
	parseUint8("batt7", &reading.Batt7)
	parseUint8("batt8", &reading.Batt8)
	parseUint8("batt9", &reading.Batt9)
	parseUint8("batt10", &reading.Batt10)
	parseUint8("batt_25", &reading.Batt25)
	parseUint8("batt_lightning", &reading.BattLightning)
	parseUint8("batleak1", &reading.BatLeak1)
	parseUint8("batleak2", &reading.BatLeak2)
	parseUint8("batleak3", &reading.BatLeak3)
	parseUint8("batleak4", &reading.BatLeak4)
	parseUint8("battsm1", &reading.BattSM1)
	parseUint8("battsm2", &reading.BattSM2)
	parseUint8("battsm3", &reading.BattSM3)
	parseUint8("battsm4", &reading.BattSM4)
	parseUint8("batt_co2", &reading.BattCO2)
	parseUint8("batt_cellgateway", &reading.BattCellGateway)

	// Pressure measurements
	parseFloat("baromrelin", &reading.BaromRelIn)
	parseFloat("baromabsin", &reading.BaromAbsIn)

	// Relay states 1-10
	parseUint8("relay1", &reading.Relay1)
	parseUint8("relay2", &reading.Relay2)
	parseUint8("relay3", &reading.Relay3)
	parseUint8("relay4", &reading.Relay4)
	parseUint8("relay5", &reading.Relay5)
	parseUint8("relay6", &reading.Relay6)
	parseUint8("relay7", &reading.Relay7)
	parseUint8("relay8", &reading.Relay8)
	parseUint8("relay9", &reading.Relay9)
	parseUint8("relay10", &reading.Relay10)

	// Air quality measurements
	parseFloat("pm25", &reading.PM25)
	parseFloat("pm25_24h", &reading.PM25_24H)
	parseFloat("pm25_in", &reading.PM25In)
	parseFloat("pm25_in_24h", &reading.PM25In24H)
	parseFloat("pm25_in_aqin", &reading.PM25InAQIN)
	parseFloat("pm25_in_24h_aqin", &reading.PM25In24HAQIN)
	parseFloat("pm10_in_aqin", &reading.PM10InAQIN)
	parseFloat("pm10_in_24h_aqin", &reading.PM10In24HAQIN)
	parseFloat("co2", &reading.CO2)
	parseInt("co2_in_aqin", &reading.CO2InAQIN)
	parseInt("co2_in_24h_aqin", &reading.CO2In24HAQIN)
	parseFloat("pm_in_temp_aqin", &reading.PMInTempAQIN)
	parseInt("pm_in_humidity_aqin", &reading.PMInHumidityAQIN)
	parseInt("aqi_pm25_aqin", &reading.AQIPM25AQIN)
	parseInt("aqi_pm25_24h_aqin", &reading.AQIPM2524HAQIN)
	parseInt("aqi_pm10_aqin", &reading.AQIPM10AQIN)
	parseInt("aqi_pm10_24h_aqin", &reading.AQIPM1024HAQIN)
	parseInt("aqi_pm25_in", &reading.AQIPM25In)
	parseInt("aqi_pm25_in_24h", &reading.AQIPM25In24H)

	// Lightning data
	parseInt("lightning_day", &reading.LightningDay)
	parseInt("lightning_hour", &reading.LightningHour)
	parseTime("lightning_time", &reading.LightningTime)
	parseFloat("lightning_distance", &reading.LightningDistance)

	// Time zone and UTC time
	parseString("tz", &reading.TZ)
	parseInt64("dateutc", &reading.DateUTC)

	// Legacy battery status
	if vals, ok := query["lowbatt"]; ok && len(vals) > 0 && vals[0] != "" {
		if parsed, err := strconv.ParseFloat(vals[0], 32); err == nil {
			reading.TxBatteryStatus = uint8(parsed)
		}
	}
}

// correctWindDirection applies the configured wind direction correction
func (s *Station) correctWindDirection(windDir float32) float32 {
	if s.config.WindDirCorrection == 0 {
		return windDir
	}

	s.logger.Debugf("Correcting wind direction by %v degrees", s.config.WindDirCorrection)
	corrected := int16(windDir) + s.config.WindDirCorrection

	// Normalize to 0-359 range
	for corrected >= 360 {
		corrected -= 360
	}
	for corrected < 0 {
		corrected += 360
	}

	return float32(corrected)
}
