package ambientcustomized

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// Station holds our HTTP server and configuration
type Station struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	server             *http.Server
	config             config.DeviceData
	configProvider     config.ConfigProvider
	deviceName         string
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
}

// NewStation creates a new Ambient Weather customized server station
func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	station := &Station{
		ctx:                ctx,
		wg:                 wg,
		configProvider:     configProvider,
		deviceName:         deviceName,
		ReadingDistributor: distributor,
		logger:             logger,
	}

	// Load configuration to get device config
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		logger.Fatalf("Ambient Weather station [%s] failed to load config: %v", deviceName, err)
	}

	// Find our device configuration
	var deviceConfig *config.DeviceData
	for _, device := range cfgData.Devices {
		if device.Name == deviceName {
			deviceConfig = &device
			break
		}
	}

	if deviceConfig == nil {
		logger.Fatalf("Ambient Weather station [%s] device not found in configuration", deviceName)
	}

	// Use the device configuration directly
	station.config = *deviceConfig

	if station.config.Port == "" {
		logger.Fatalf("Ambient Weather station [%s] must define a port", station.config.Name)
	}

	return station
}

func (s *Station) StationName() string {
	return s.config.Name
}

// StartWeatherStation starts the HTTP server to listen for weather data
func (s *Station) StartWeatherStation() error {
	log.Infof("Starting Ambient Weather customized server station [%v] on port %s...", s.config.Name, s.config.Port)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWeatherUpdate)

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
			log.Errorf("Ambient Weather station [%s] HTTP server error: %v", s.config.Name, err)
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-s.ctx.Done()
		log.Infof("Shutting down Ambient Weather station [%s] HTTP server...", s.config.Name)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.Errorf("Ambient Weather station [%s] HTTP server shutdown error: %v", s.config.Name, err)
		}
	}()

	return nil
}

// handleWeatherUpdate processes incoming weather data from the Ambient Weather station
func (s *Station) handleWeatherUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Validate required parameters
	stationID := query.Get("ID")
	password := query.Get("PASSWORD")
	dateUTC := query.Get("dateutc")
	action := query.Get("action")

	if stationID == "" || password == "" || dateUTC == "" || action != "updateraw" {
		log.Debugf("Ambient Weather station [%s] received invalid request: missing required parameters", s.config.Name)
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Parse timestamp
	timestamp, err := time.Parse("2006-01-02 15:04:05", dateUTC)
	if err != nil {
		log.Debugf("Ambient Weather station [%s] failed to parse timestamp '%s': %v", s.config.Name, dateUTC, err)
		http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
		return
	}

	// Convert to local time (weather stations typically send UTC)
	timestamp = timestamp.UTC()

	// Create reading
	reading := types.Reading{
		Timestamp:   timestamp,
		StationName: s.config.Name,
		StationType: "ambient-customized",
	}

	// Parse and set weather data
	if val := query.Get("tempf"); val != "" {
		if temp, err := strconv.ParseFloat(val, 32); err == nil {
			reading.OutTemp = float32(temp)
		}
	}

	if val := query.Get("humidity"); val != "" {
		if humidity, err := strconv.ParseFloat(val, 32); err == nil {
			reading.OutHumidity = float32(humidity)
		}
	}

	if val := query.Get("dewptf"); val != "" {
		if dewpt, err := strconv.ParseFloat(val, 32); err == nil {
			reading.ExtraTemp1 = float32(dewpt) // Store dew point in ExtraTemp1
		}
	}

	if val := query.Get("winddir"); val != "" {
		if winddir, err := strconv.ParseFloat(val, 32); err == nil {
			reading.WindDir = float32(winddir)
		}
	}

	if val := query.Get("windspeedmph"); val != "" {
		if windspeed, err := strconv.ParseFloat(val, 32); err == nil {
			reading.WindSpeed = float32(windspeed)
		}
	}

	if val := query.Get("windgustmph"); val != "" {
		if windgust, err := strconv.ParseFloat(val, 32); err == nil {
			reading.WindSpeed10 = float32(windgust) // Store wind gust in WindSpeed10
		}
	}

	if val := query.Get("rainin"); val != "" {
		if rain, err := strconv.ParseFloat(val, 32); err == nil {
			reading.RainIncremental = float32(rain)
		}
	}

	if val := query.Get("dailyrainin"); val != "" {
		if dayrain, err := strconv.ParseFloat(val, 32); err == nil {
			reading.DayRain = float32(dayrain)
		}
	}

	if val := query.Get("weeklyrainin"); val != "" {
		if weekrain, err := strconv.ParseFloat(val, 32); err == nil {
			reading.ExtraFloat1 = float32(weekrain) // Store weekly rain in ExtraFloat1
		}
	}

	if val := query.Get("monthlyrainin"); val != "" {
		if monthrain, err := strconv.ParseFloat(val, 32); err == nil {
			reading.MonthRain = float32(monthrain)
		}
	}

	if val := query.Get("totalrainin"); val != "" {
		if totalrain, err := strconv.ParseFloat(val, 32); err == nil {
			reading.YearRain = float32(totalrain) // Store total rain in YearRain
		}
	}

	if val := query.Get("solarradiation"); val != "" {
		if solar, err := strconv.ParseFloat(val, 32); err == nil {
			reading.SolarWatts = float32(solar)
		}
	}

	if val := query.Get("UV"); val != "" {
		if uv, err := strconv.ParseFloat(val, 32); err == nil {
			reading.UV = float32(uv)
		}
	}

	if val := query.Get("indoortempf"); val != "" {
		if intemp, err := strconv.ParseFloat(val, 32); err == nil {
			reading.InTemp = float32(intemp)
		}
	}

	if val := query.Get("indoorhumidity"); val != "" {
		if inhumidity, err := strconv.ParseFloat(val, 32); err == nil {
			reading.InHumidity = float32(inhumidity)
		}
	}

	if val := query.Get("baromin"); val != "" {
		if baro, err := strconv.ParseFloat(val, 32); err == nil {
			reading.Barometer = float32(baro)
		}
	}

	if val := query.Get("lowbatt"); val != "" {
		if lowbatt, err := strconv.ParseFloat(val, 32); err == nil {
			reading.TxBatteryStatus = uint8(lowbatt)
		}
	}

	// Calculate derived values
	reading.WindChill = s.calculateWindChill(reading.OutTemp, reading.WindSpeed)
	reading.HeatIndex = s.calculateHeatIndex(reading.OutTemp, reading.OutHumidity)

	// Send the reading to the distributor
	log.Debugf("Ambient Weather station [%s] received update: temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph @ %.0f°, pressure=%.2f\"",
		s.config.Name, reading.OutTemp, reading.OutHumidity, reading.WindSpeed, reading.WindDir, reading.Barometer)

	s.ReadingDistributor <- reading

	// Respond with success
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("success"))
}

// calculateWindChill calculates wind chill temperature
func (s *Station) calculateWindChill(tempF, windMPH float32) float32 {
	if tempF > 50.0 || windMPH < 3.0 {
		return tempF
	}

	// Wind chill formula (US National Weather Service)
	windChill := 35.74 + 0.6215*tempF - 35.75*float32(math.Pow(float64(windMPH), 0.16)) + 0.4275*tempF*float32(math.Pow(float64(windMPH), 0.16))
	return windChill
}

// calculateHeatIndex calculates heat index temperature
func (s *Station) calculateHeatIndex(tempF, humidity float32) float32 {
	if tempF < 80.0 {
		return tempF
	}

	// Heat index formula (US National Weather Service)
	c1 := -42.379
	c2 := 2.04901523
	c3 := 10.14333127
	c4 := -0.22475541
	c5 := -0.00683783
	c6 := -0.05481717
	c7 := 0.00122874
	c8 := 0.00085282
	c9 := -0.00000199

	T := float64(tempF)
	R := float64(humidity)

	heatIndex := c1 + c2*T + c3*R + c4*T*R + c5*T*T + c6*R*R + c7*T*T*R + c8*T*R*R + c9*T*T*R*R

	return float32(heatIndex)
}
