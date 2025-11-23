package weatherlinklive

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// Station represents a WeatherLink Live weather station
type Station struct {
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 *sync.WaitGroup
	config             config.DeviceData
	mappings           []SensorMapping
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger

	// UDP broadcast state (only if wll_broadcast=true)
	udpConn         *net.UDPConn
	broadcastPort   int
	broadcastExpiry time.Time

	// Connection state
	connected   bool
	connectedMu sync.RWMutex
}

// NewStation creates a new WeatherLink Live station instance
func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	// Validate network config (WLL is network-only)
	if deviceConfig.Hostname == "" {
		logger.Fatal("WeatherLink Live requires hostname configuration")
	}

	// Create child context
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

// StationName returns the station name
func (s *Station) StationName() string {
	return s.config.Name
}

// Capabilities returns the station capabilities
func (s *Station) Capabilities() weatherstations.Capabilities {
	return weatherstations.Capabilities(weatherstations.Weather)
}

// StartWeatherStation starts the weather station data collection
func (s *Station) StartWeatherStation() error {
	s.logger.Infof("Starting WeatherLink Live station [%s]", s.config.Name)

	// Parse mapping configuration
	mappings, err := ParseMappingString(s.config.WLLSensorMapping)
	if err != nil {
		return fmt.Errorf("invalid sensor mapping: %w", err)
	}
	s.mappings = mappings
	s.logger.Debugf("Parsed %d sensor mappings", len(mappings))

	// Start mode-specific collection
	if s.config.WLLBroadcast {
		s.logger.Info("Starting UDP broadcast mode")
		if err := s.startBroadcastMode(); err != nil {
			return fmt.Errorf("failed to start broadcast mode: %w", err)
		}
	} else {
		s.logger.Info("Starting HTTP polling mode")
	}

	// Start collection goroutine
	s.wg.Add(1)
	go s.collectData()

	return nil
}

// StopWeatherStation stops the weather station data collection
func (s *Station) StopWeatherStation() error {
	s.logger.Infof("Stopping WeatherLink Live station [%s]", s.config.Name)

	// Cancel context first
	s.cancel()

	// Close UDP connection if active
	if s.udpConn != nil {
		s.udpConn.Close()
	}

	return nil
}

// collectData is the main data collection goroutine
func (s *Station) collectData() {
	defer s.wg.Done()

	if s.config.WLLBroadcast {
		// UDP broadcast mode - wait for context cancellation
		<-s.ctx.Done()
	} else {
		// HTTP polling mode
		s.runPollingMode()
	}
}

// startBroadcastMode initiates UDP broadcast mode
func (s *Station) startBroadcastMode() error {
	// Start real-time broadcast
	resp, err := StartRealTimeBroadcast(s.ctx, s.config.Hostname, 3600)
	if err != nil {
		return fmt.Errorf("failed to start real-time broadcast: %w", err)
	}

	s.broadcastPort = resp.Data.BroadcastPort
	s.broadcastExpiry = time.Now().Add(time.Duration(resp.Data.Duration) * time.Second)

	s.logger.Infof("Real-time broadcast started on port %d, expires at %s", s.broadcastPort, s.broadcastExpiry.Format(time.RFC3339))

	// Start UDP receiver
	if err := s.startUDPReceiver(); err != nil {
		return fmt.Errorf("failed to start UDP receiver: %w", err)
	}

	// Schedule broadcast refresh
	s.wg.Add(1)
	go s.refreshBroadcast()

	return nil
}

// refreshBroadcast periodically refreshes the broadcast registration
func (s *Station) refreshBroadcast() {
	defer s.wg.Done()

	for {
		// Calculate when to refresh (90% of duration)
		timeUntilRefresh := time.Until(s.broadcastExpiry) * 9 / 10
		if timeUntilRefresh < time.Minute {
			timeUntilRefresh = time.Minute
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(timeUntilRefresh):
			s.logger.Debug("Refreshing real-time broadcast registration")

			resp, err := StartRealTimeBroadcast(s.ctx, s.config.Hostname, 3600)
			if err != nil {
				s.logger.Errorf("Failed to refresh broadcast: %v", err)
				s.setConnected(false)
				continue
			}

			// Check if port changed
			if resp.Data.BroadcastPort != s.broadcastPort {
				s.logger.Infof("Broadcast port changed from %d to %d", s.broadcastPort, resp.Data.BroadcastPort)

				// Close old UDP connection
				if s.udpConn != nil {
					s.udpConn.Close()
				}

				// Update port and restart receiver
				s.broadcastPort = resp.Data.BroadcastPort
				if err := s.startUDPReceiver(); err != nil {
					s.logger.Errorf("Failed to restart UDP receiver: %v", err)
					s.setConnected(false)
					continue
				}
			}

			s.broadcastExpiry = time.Now().Add(time.Duration(resp.Data.Duration) * time.Second)
			s.logger.Debugf("Broadcast refreshed, expires at %s", s.broadcastExpiry.Format(time.RFC3339))
			s.setConnected(true)
		}
	}
}

// runPollingMode runs HTTP polling mode
func (s *Station) runPollingMode() {
	// Default poll interval
	pollInterval := 60 * time.Second
	if s.config.WLLPollInterval > 0 {
		pollInterval = time.Duration(s.config.WLLPollInterval) * time.Second
	}

	// Minimum 30 seconds to avoid overwhelming device
	if pollInterval < 30*time.Second {
		pollInterval = 30 * time.Second
		s.logger.Warnf("Poll interval too short, using minimum of 30 seconds")
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	s.logger.Infof("HTTP polling mode started with %v interval", pollInterval)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			resp, err := GetCurrentConditions(s.ctx, s.config.Hostname)
			if err != nil {
				s.logger.Errorf("Failed to get current conditions: %v", err)
				s.setConnected(false)
				continue
			}

			s.setConnected(true)
			reading := s.transformToReading(&resp.Data)
			s.ReadingDistributor <- *reading
		}
	}
}

// setConnected sets the connected state
func (s *Station) setConnected(connected bool) {
	s.connectedMu.Lock()
	defer s.connectedMu.Unlock()
	s.connected = connected
}

// isConnected returns the connected state
func (s *Station) isConnected() bool {
	s.connectedMu.RLock()
	defer s.connectedMu.RUnlock()
	return s.connected
}
