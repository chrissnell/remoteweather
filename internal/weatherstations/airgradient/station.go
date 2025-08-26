package airgradient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// AirGradientResponse represents the JSON response from /measures/current endpoint
type AirGradientResponse struct {
	PM01             float32 `json:"pm01"`             // PM1.0 µg/m³
	PM02             float32 `json:"pm02"`             // PM2.5 µg/m³ (raw)
	PM10             float32 `json:"pm10"`             // PM10 µg/m³
	PM02Compensated  float32 `json:"pm02Compensated"`  // PM2.5 compensated µg/m³
	PM003Count       float32 `json:"pm003Count"`       // 0.3µm particles/100ml
	PM005Count       float32 `json:"pm005Count"`       // 0.5µm particles/100ml
	PM01Count        float32 `json:"pm01Count"`        // 1.0µm particles/100ml
	PM02Count        float32 `json:"pm02Count"`        // 2.5µm particles/100ml
	Atmp             float32 `json:"atmp"`             // Temperature °C (raw)
	AtmpCompensated  float32 `json:"atmpCompensated"`  // Temperature °C (compensated)
	Rhum             float32 `json:"rhum"`             // Humidity % (raw)
	RhumCompensated  float32 `json:"rhumCompensated"`  // Humidity % (compensated)
	Rco2             float32 `json:"rco2"`             // CO2 ppm
	TvocIndex        float32 `json:"tvocIndex"`        // TVOC index (1-500)
	TvocRaw          float32 `json:"tvocRaw"`          // TVOC raw value
	NoxIndex         float32 `json:"noxIndex"`         // NOx index (1-500)
	NoxRaw           float32 `json:"noxRaw"`           // NOx raw value
	Wifi             float32 `json:"wifi"`             // WiFi RSSI dBm
	SerialNo         string  `json:"serialno"`         // Device serial number
	Firmware         string  `json:"firmware"`         // Firmware version
	Model            string  `json:"model"`            // Device model
}

type Station struct {
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 *sync.WaitGroup
	config             config.DeviceData
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
	client             *http.Client
	pollInterval       time.Duration
}

// NewStation creates a new AirGradient weather station instance
func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	stationCtx, cancel := context.WithCancel(ctx)
	
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	// Set default port if not specified
	if deviceConfig.Port == "" {
		deviceConfig.Port = "80"
	}

	// Default poll interval of 3 seconds (AirGradient supports fast polling)
	pollInterval := 3 * time.Second
	
	station := &Station{
		ctx:                stationCtx,
		cancel:             cancel,
		wg:                 wg,
		config:             *deviceConfig,
		ReadingDistributor: distributor,
		logger:             logger.Named("airgradient").With("station", deviceName),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		pollInterval: pollInterval,
	}

	return station
}

// StationName returns the name of this weather station
func (s *Station) StationName() string {
	return s.config.Name
}

// StartWeatherStation starts polling the AirGradient device
func (s *Station) StartWeatherStation() error {
	if s.config.Hostname == "" {
		return fmt.Errorf("hostname is required for AirGradient station")
	}

	s.logger.Infow("Starting AirGradient station",
		"hostname", s.config.Hostname,
		"port", s.config.Port,
		"interval", s.pollInterval)

	s.wg.Add(1)
	go s.pollLoop()

	return nil
}

// StopWeatherStation stops the weather station
func (s *Station) StopWeatherStation() error {
	s.logger.Info("Stopping AirGradient station")
	s.cancel()
	return nil
}

func (s *Station) pollLoop() {
	defer s.wg.Done()

	// Initial poll immediately
	s.pollDevice()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Poll loop stopped")
			return
		case <-ticker.C:
			s.pollDevice()
		}
	}
}

func (s *Station) pollDevice() {
	url := fmt.Sprintf("http://%s:%s/measures/current", s.config.Hostname, s.config.Port)
	
	resp, err := s.client.Get(url)
	if err != nil {
		s.logger.Errorw("Failed to fetch AirGradient data", "error", err, "url", url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Errorw("Unexpected status code from AirGradient", 
			"status", resp.StatusCode, 
			"url", url)
		return
	}

	var data AirGradientResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		s.logger.Errorw("Failed to decode AirGradient response", "error", err)
		return
	}

	reading := s.convertToReading(data)
	
	select {
	case s.ReadingDistributor <- reading:
		s.logger.Debugw("Reading sent", 
			"temp", reading.OutTemp,
			"humidity", reading.OutHumidity,
			"co2", reading.CO2,
			"pm25", reading.PM25)
	case <-s.ctx.Done():
		return
	}
}

func (s *Station) convertToReading(data AirGradientResponse) types.Reading {
	reading := types.Reading{
		Timestamp:   time.Now(),
		StationName: s.config.Name,
		StationType: "airgradient",
		
		// Core environmental data - use compensated values when available
		OutTemp:     celsiusToFahrenheit(data.AtmpCompensated),
		OutHumidity: data.RhumCompensated,
		CO2:         data.Rco2,
		PM25:        data.PM02Compensated, // Use compensated PM2.5
		
		// Additional air quality data in ExtraFloat fields
		ExtraFloat1: data.PM01,       // PM1.0 µg/m³
		ExtraFloat2: data.PM10,       // PM10 µg/m³
		ExtraFloat3: data.TvocIndex,  // TVOC index
		ExtraFloat4: data.NoxIndex,   // NOx index
		ExtraFloat5: data.Wifi,       // WiFi RSSI dBm
		ExtraFloat6: data.PM003Count, // 0.3µm particle count
		ExtraFloat7: data.PM005Count, // 0.5µm particle count
		ExtraFloat8: data.PM01Count,  // 1.0µm particle count
		ExtraFloat9: data.PM02Count,  // 2.5µm particle count
		
		// Device metadata
		ExtraText1: data.SerialNo, // Serial number
		ExtraText2: data.Model,    // Model
		ExtraText3: data.Firmware, // Firmware version
	}
	
	return reading
}

func celsiusToFahrenheit(celsius float32) float32 {
	return (celsius * 9.0 / 5.0) + 32.0
}