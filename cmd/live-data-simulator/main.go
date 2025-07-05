package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
)

const (
	defaultInterval = 4 * time.Second
	defaultConfigDB = "config.db"
	liveDataURL     = "https://suncrestweather.com/latest?station=CSI"
)

// LiveWeatherData represents the structure of data from suncrestweather.com
type LiveWeatherData struct {
	StationName           string  `json:"stationname"`
	StationType           string  `json:"stationtype"`
	Timestamp             int64   `json:"ts"`
	OutTemp               float64 `json:"otemp"`
	ExtraTemp1            float64 `json:"extratemp1"`
	OutHumidity           float64 `json:"outhumidity"`
	RainRate              float64 `json:"rainrate"`
	RainIncremental       float64 `json:"rainincremental"`
	SolarWatts            float64 `json:"solarwatts"`
	PotentialSolarWatts   float64 `json:"potentialsolarwatts"`
	SolarJoules           float64 `json:"solarjoules"`
	Barometer             float64 `json:"bar"`
	WindSpeed             float64 `json:"winds"`
	WindDir               float64 `json:"windd"`
	WindCard              string  `json:"windcard"`
	StationBatteryVoltage float64 `json:"stationbatteryvoltage"`
}

// CampbellPacket represents the Campbell Scientific format expected by remoteweather
type CampbellPacket struct {
	StationBatteryVoltage float32 `json:"batt_volt,omitempty"`
	OutTemp               float32 `json:"airtemp_f,omitempty"`
	OutHumidity           float32 `json:"rh,omitempty"`
	Barometer             float32 `json:"baro,omitempty"`
	ExtraTemp1            float32 `json:"baro_temp_f,omitempty"`
	SolarWatts            float32 `json:"slr_w,omitempty"`
	SolarJoules           float32 `json:"slr_mj,omitempty"`
	RainIncremental       float32 `json:"rain_in,omitempty"`
	WindSpeed             float32 `json:"wind_s,omitempty"`
	WindDir               uint16  `json:"wind_d,omitempty"`
}

// StationServer represents a TCP server for a specific weather station
type StationServer struct {
	station     config.DeviceData
	listener    net.Listener
	currentData *CampbellPacket
	dataMutex   sync.RWMutex

	// Consistent variations applied to each reading
	tempVariation      float64
	humidityVariation  float64
	windSpeedVariation float64
	pressureVariation  float64
	windDirVariation   float64
	batteryVariation   float64
}

// LiveDataSimulator manages the live data fetching and station servers
type LiveDataSimulator struct {
	configProvider config.ConfigProvider
	httpClient     *http.Client
	liveData       *LiveWeatherData
	dataMutex      sync.RWMutex
	stationServers map[string]*StationServer
	serversMutex   sync.RWMutex
	logger         *log.Logger
}

func main() {
	var (
		configFile = flag.String("config", defaultConfigDB, "Path to configuration database")
		interval   = flag.Duration("interval", defaultInterval, "Interval between data fetches")
		basePort   = flag.Int("base-port", 7100, "Base port for station servers (each station gets port+offset)")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[live-data-simulator] ", log.LstdFlags)

	// Create configuration provider
	configProvider, err := createConfigProvider(*configFile)
	if err != nil {
		logger.Fatalf("Failed to create config provider: %v", err)
	}
	defer configProvider.Close()

	// Create simulator
	simulator := &LiveDataSimulator{
		configProvider: configProvider,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		stationServers: make(map[string]*StationServer),
		logger:         logger,
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Println("Shutdown signal received, stopping...")
		cancel()
	}()

	// Start the simulator
	if err := simulator.Start(ctx, *interval, *basePort); err != nil {
		logger.Fatalf("Failed to start simulator: %v", err)
	}
}

func createConfigProvider(configFile string) (config.ConfigProvider, error) {
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file does not exist: %s", configFile)
	}

	// Create SQLite provider
	provider, err := config.NewSQLiteProvider(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite provider: %w", err)
	}

	// Test that we can load the config
	_, err = provider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Wrap with caching
	return config.NewCachedProvider(provider, 30*time.Second), nil
}

func (s *LiveDataSimulator) Start(ctx context.Context, interval time.Duration, basePort int) error {
	s.logger.Printf("Starting live data simulator with %v interval", interval)

	// Start data fetcher
	go s.fetchLiveData(ctx, interval)

	// Start station servers
	if err := s.startStationServers(ctx, basePort); err != nil {
		return fmt.Errorf("failed to start station servers: %w", err)
	}

	// Wait for shutdown
	<-ctx.Done()
	s.logger.Println("Shutting down...")
	s.stopStationServers()
	return nil
}

func (s *LiveDataSimulator) fetchLiveData(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Printf("Starting live data fetcher from %s", liveDataURL)

	// Fetch initial data
	s.fetchAndUpdateData()

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("Data fetcher shutting down")
			return
		case <-ticker.C:
			s.fetchAndUpdateData()
		}
	}
}

func (s *LiveDataSimulator) fetchAndUpdateData() {
	req, err := http.NewRequest("GET", liveDataURL, nil)
	if err != nil {
		s.logger.Printf("Failed to create request: %v", err)
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Printf("Failed to fetch live data: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Printf("HTTP error fetching live data: %d", resp.StatusCode)
		return
	}

	var liveData LiveWeatherData
	if err := json.NewDecoder(resp.Body).Decode(&liveData); err != nil {
		s.logger.Printf("Failed to decode live data: %v", err)
		return
	}

	// Update stored data
	s.dataMutex.Lock()
	s.liveData = &liveData
	s.dataMutex.Unlock()

	// Update all station servers with skewed data
	s.updateStationData()

	s.logger.Printf("Updated live data: temp=%.1f°F, humidity=%.1f%%, wind=%.1f@%.0f°",
		liveData.OutTemp, liveData.OutHumidity, liveData.WindSpeed, liveData.WindDir)
}

func (s *LiveDataSimulator) startStationServers(ctx context.Context, basePort int) error {
	// Get all stations from config
	devices, err := s.configProvider.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	// Filter for enabled stations
	var enabledStations []config.DeviceData
	for _, device := range devices {
		if device.Enabled {
			enabledStations = append(enabledStations, device)
		}
	}

	if len(enabledStations) == 0 {
		s.logger.Println("No enabled stations found in configuration")
		return nil
	}

	s.logger.Printf("Starting servers for %d stations", len(enabledStations))

	// Start server for each station
	for i, station := range enabledStations {
		port := basePort + i

		if err := s.startStationServer(ctx, station, port); err != nil {
			s.logger.Printf("Failed to start server for station %s: %v", station.Name, err)
			continue
		}
	}

	return nil
}

func (s *LiveDataSimulator) startStationServer(ctx context.Context, station config.DeviceData, port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	// Generate consistent variations for this station
	tempVar := (rand.Float64() - 0.5) * 1.0
	humidityVar := (rand.Float64() - 0.5) * 4.0
	windSpeedVar := (rand.Float64() - 0.5) * 2.0
	pressureVar := (rand.Float64() - 0.5) * 0.04
	windDirVar := (rand.Float64() - 0.5) * 10.0
	batteryVar := (rand.Float64() - 0.5) * 0.2

	server := &StationServer{
		station:            station,
		listener:           listener,
		tempVariation:      tempVar,
		humidityVariation:  humidityVar,
		windSpeedVariation: windSpeedVar,
		pressureVariation:  pressureVar,
		windDirVariation:   windDirVar,
		batteryVariation:   batteryVar,
	}

	s.serversMutex.Lock()
	s.stationServers[station.Name] = server
	s.serversMutex.Unlock()

	s.logger.Printf("Started server for station %s on port %d (altitude: %.0fm)", station.Name, port, station.Altitude)

	// Start accepting connections
	go s.handleStationConnections(ctx, server)

	return nil
}

func (s *LiveDataSimulator) handleStationConnections(ctx context.Context, server *StationServer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := server.listener.Accept()
			if err != nil {
				// Check if we're shutting down
				select {
				case <-ctx.Done():
					return
				default:
					s.logger.Printf("Failed to accept connection for station %s: %v", server.station.Name, err)
					continue
				}
			}

			s.logger.Printf("New connection for station %s from %s", server.station.Name, conn.RemoteAddr())
			go s.handleStationConnection(ctx, server, conn)
		}
	}
}

func (s *LiveDataSimulator) handleStationConnection(ctx context.Context, server *StationServer, conn net.Conn) {
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	ticker := time.NewTicker(2 * time.Second) // Send data every 2 seconds
	defer ticker.Stop()

	// Extract port from connection address for logging
	serverAddr := server.listener.Addr().String()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			server.dataMutex.RLock()
			data := server.currentData
			server.dataMutex.RUnlock()

			if data != nil {
				if err := encoder.Encode(data); err != nil {
					s.logger.Printf("Failed to send data to station %s: %v", server.station.Name, err)
					return
				}
				s.logger.Printf("[%s] %s: temp=%.1f°F, humidity=%.1f%%, wind=%.1f@%d°, pressure=%.2f\"",
					serverAddr, server.station.Name, data.OutTemp, data.OutHumidity,
					data.WindSpeed, data.WindDir, data.Barometer)
			} else {
				// Send default data if we don't have live data yet
				defaultData := &CampbellPacket{
					StationBatteryVoltage: 13.2,
					OutTemp:               72.0,
					OutHumidity:           50.0,
					Barometer:             30.0,
					ExtraTemp1:            75.0,
					SolarWatts:            0.0,
					SolarJoules:           0.0,
					RainIncremental:       0.0,
					WindSpeed:             5.0,
					WindDir:               180,
				}
				if err := encoder.Encode(defaultData); err != nil {
					s.logger.Printf("Failed to send default data to station %s: %v", server.station.Name, err)
					return
				}
				s.logger.Printf("[%s] %s: temp=%.1f°F, humidity=%.1f%%, wind=%.1f@%d° (DEFAULT DATA)",
					serverAddr, server.station.Name, defaultData.OutTemp, defaultData.OutHumidity,
					defaultData.WindSpeed, defaultData.WindDir)
			}
		}
	}
}

func (s *LiveDataSimulator) updateStationData() {
	s.dataMutex.RLock()
	liveData := s.liveData
	s.dataMutex.RUnlock()

	if liveData == nil {
		return
	}

	s.serversMutex.RLock()
	defer s.serversMutex.RUnlock()

	for _, server := range s.stationServers {
		skewedData := s.applySkewing(liveData, server)

		server.dataMutex.Lock()
		server.currentData = skewedData
		server.dataMutex.Unlock()
	}
}

func (s *LiveDataSimulator) applySkewing(liveData *LiveWeatherData, server *StationServer) *CampbellPacket {
	// Reference altitude for the live data source (1900m)
	const referenceAltitude = 1900.0

	// Use the station's altitude directly
	stationAltitude := server.station.Altitude

	// Calculate temperature adjustment based on altitude difference
	// Standard atmospheric lapse rate: ~2°C per 1000m (3.5°F per 1000m)
	altitudeDiff := stationAltitude - referenceAltitude
	tempAdjustment := -(altitudeDiff / 1000.0) * 3.5 // Cooler at higher altitude, warmer at lower

	// Use consistent variations that were generated once per station
	tempVariation := server.tempVariation
	humidityVariation := server.humidityVariation
	windSpeedVariation := server.windSpeedVariation
	pressureVariation := server.pressureVariation
	windDirVariation := server.windDirVariation
	batteryVariation := server.batteryVariation

	// Apply temperature adjustment (altitude-based + small random variation)
	adjustedTemp := liveData.OutTemp + tempAdjustment + tempVariation

	// Apply small variations to other parameters (no multiplicative skewing)
	adjustedHumidity := liveData.OutHumidity + humidityVariation
	adjustedWindSpeed := liveData.WindSpeed + windSpeedVariation
	adjustedWindDir := liveData.WindDir + windDirVariation

	// Apply bounds checking
	if adjustedHumidity < 0 {
		adjustedHumidity = 0
	} else if adjustedHumidity > 100 {
		adjustedHumidity = 100
	}

	if adjustedWindSpeed < 0 {
		adjustedWindSpeed = 0
	}

	// Normalize wind direction to 0-360 degrees
	for adjustedWindDir < 0 {
		adjustedWindDir += 360
	}
	for adjustedWindDir >= 360 {
		adjustedWindDir -= 360
	}

	return &CampbellPacket{
		StationBatteryVoltage: float32(liveData.StationBatteryVoltage + batteryVariation), // Use consistent variation
		OutTemp:               float32(adjustedTemp),
		OutHumidity:           float32(adjustedHumidity),
		Barometer:             float32(liveData.Barometer + pressureVariation),
		ExtraTemp1:            float32(liveData.ExtraTemp1 + tempAdjustment + tempVariation),
		SolarWatts:            float32(liveData.SolarWatts),      // No variation for solar
		SolarJoules:           float32(liveData.SolarJoules),     // No variation for solar
		RainIncremental:       float32(liveData.RainIncremental), // No variation for rain
		WindSpeed:             float32(adjustedWindSpeed),
		WindDir:               uint16(adjustedWindDir),
	}
}

func (s *LiveDataSimulator) stopStationServers() {
	s.serversMutex.Lock()
	defer s.serversMutex.Unlock()

	for name, server := range s.stationServers {
		if server.listener != nil {
			server.listener.Close()
			s.logger.Printf("Stopped server for station %s", name)
		}
	}
}
