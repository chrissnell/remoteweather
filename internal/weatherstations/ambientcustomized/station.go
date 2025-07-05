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
	s.logger.Infof("Starting Ambient Weather customized server station [%s] on port %s", s.config.Name, s.config.Port)

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

	parseFloat("tempf", &reading.OutTemp)
	parseFloat("humidity", &reading.OutHumidity)
	parseFloat("dewptf", &reading.ExtraTemp1)
	parseFloat("winddir", &reading.WindDir)
	parseFloat("windspeedmph", &reading.WindSpeed)
	parseFloat("windgustmph", &reading.WindSpeed10)
	parseFloat("rainin", &reading.RainIncremental)
	parseFloat("dailyrainin", &reading.DayRain)
	parseFloat("weeklyrainin", &reading.ExtraFloat1)
	parseFloat("monthlyrainin", &reading.MonthRain)
	parseFloat("totalrainin", &reading.YearRain)
	parseFloat("solarradiation", &reading.SolarWatts)
	parseFloat("UV", &reading.UV)
	parseFloat("indoortempf", &reading.InTemp)
	parseFloat("indoorhumidity", &reading.InHumidity)
	parseFloat("baromin", &reading.Barometer)

	if vals, ok := query["lowbatt"]; ok && len(vals) > 0 && vals[0] != "" {
		if parsed, err := strconv.ParseFloat(vals[0], 32); err == nil {
			reading.TxBatteryStatus = uint8(parsed)
		}
	}
}
