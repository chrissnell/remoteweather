// Package campbell provides Campbell Scientific weather station support with JSON communication.
package campbell

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/solar"
	serial "github.com/tarm/goserial"
	"go.uber.org/zap"
)

const (
	defaultBaud      = 19200
	networkTimeout   = 30 * time.Second
	retryDelay       = 5 * time.Second
	serialRetryDelay = 30 * time.Second
)

type Station struct {
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 *sync.WaitGroup
	netConn            net.Conn
	rwc                io.ReadWriteCloser
	config             config.DeviceData
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
	connecting         bool
	connectingMu       sync.RWMutex
	connected          bool
	connectedMu        sync.RWMutex
}

type Packet struct {
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

func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	if err := weatherstations.ValidateSerialOrNetwork(*deviceConfig); err != nil {
		logger.Fatal(err)
	}

	if deviceConfig.Baud == 0 {
		deviceConfig.Baud = defaultBaud
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
	s.logger.Infof("Starting Campbell Scientific weather station [%s]", s.config.Name)

	s.ConnectToStation()

	s.wg.Add(1)
	go s.GetCampbellScientificPackets()

	return nil
}

func (s *Station) StopWeatherStation() error {
	s.logger.Infof("Stopping Campbell Scientific weather station [%s]", s.config.Name)
	s.cancel()

	// Close connections
	if s.rwc != nil {
		s.rwc.Close()
	}
	if s.netConn != nil {
		s.netConn.Close()
	}

	return nil
}

func (s *Station) ConnectToStation() {
	s.Connect()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received while waiting for first packet")
			return
		default:
		}

		s.logger.Infof("Waiting for first packet from station [%s]", s.config.Name)

		if s.rwc == nil {
			s.logger.Info("connection is nil, attempting to reconnect")
			s.Connect()
			continue
		}

		dec := json.NewDecoder(s.rwc)
		packet := new(Packet)
		if err := dec.Decode(&packet); err != nil {
			s.logger.Debugf("error decoding JSON from station: %v", err)
			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				return
			case <-time.After(500 * time.Millisecond):
			}
		} else {
			s.logger.Infof("Station [%s] is alive", s.config.Name)
			return
		}
	}
}

func (s *Station) GetCampbellScientificPackets() {
	defer s.wg.Done()
	s.logger.Info("starting Campbell Scientific packet getter")

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received")
			return
		default:
			if err := s.ParseCampbellScientificPackets(); err != nil {
				s.logger.Error(err)
				s.rwc.Close()
				if s.netConn != nil {
					s.netConn.Close()
				}
				s.logger.Info("attempting to reconnect")
				s.Connect()
			} else {
				return
			}
		}
	}
}

func (s *Station) ParseCampbellScientificPackets() error {
	var packet Packet
	scanner := bufio.NewScanner(s.rwc)

	for scanner.Scan() {
		if s.netConn != nil {
			s.netConn.SetReadDeadline(time.Now().Add(networkTimeout))
		}

		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received")
			return nil
		default:
		}

		if err := json.Unmarshal(scanner.Bytes(), &packet); err != nil {
			return fmt.Errorf("error unmarshalling JSON: %v", err)
		}

		windDir := s.correctWindDirection(packet.WindDir)
		timestamp := time.Now()

		var potentialSolarWatts float64
		if s.config.Latitude != 0 && s.config.Longitude != 0 {
			potentialSolarWatts = solar.CalculateClearSkySolarRadiationASCE(
				timestamp, s.config.Latitude, s.config.Longitude,
				s.config.Altitude, float64(packet.OutTemp), float64(packet.OutHumidity))
			s.logger.Debugf("solar calculation results: %+v", potentialSolarWatts)
		}

		reading := types.Reading{
			Timestamp:             timestamp,
			StationName:           s.config.Name,
			StationType:           "campbell",
			StationBatteryVoltage: packet.StationBatteryVoltage,
			OutTemp:               packet.OutTemp,
			OutHumidity:           packet.OutHumidity,
			Barometer:             packet.Barometer,
			ExtraTemp1:            packet.ExtraTemp1,
			SolarWatts:            packet.SolarWatts,
			SolarJoules:           packet.SolarJoules,
			RainIncremental:       packet.RainIncremental,
			WindSpeed:             packet.WindSpeed,
			WindDir:               windDir,
			WindChill:             weatherstations.CalculateWindChill(packet.OutTemp, packet.WindSpeed),
			HeatIndex:             weatherstations.CalculateHeatIndex(packet.OutTemp, packet.OutHumidity),
			PotentialSolarWatts:   float32(potentialSolarWatts),
		}

		s.logger.Debugf("Campbell Scientific [%s] sending reading: temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph @ %d°, pressure=%.2f\"",
			s.config.Name, reading.OutTemp, reading.OutHumidity, reading.WindSpeed, int(reading.WindDir), reading.Barometer)
		s.ReadingDistributor <- reading
	}

	return fmt.Errorf("scanning aborted due to error or EOF")
}

func (s *Station) correctWindDirection(windDir uint16) float32 {
	corrected := int16(windDir)

	if s.config.WindDirCorrection != 0 {
		s.logger.Debugf("correcting wind direction by %v", s.config.WindDirCorrection)
		corrected += s.config.WindDirCorrection

		if corrected >= 360 {
			corrected -= 360
		} else if corrected < 0 {
			corrected += 360
		}
	}

	return float32(corrected)
}

func (s *Station) Connect() {
	if s.config.SerialDevice != "" {
		s.connectToSerialStation()
	} else {
		s.connectToNetworkStation()
	}
}

func (s *Station) connectToSerialStation() {
	s.connectingMu.RLock()
	if s.connecting {
		s.connectingMu.RUnlock()
		s.logger.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}
	s.connectingMu.RUnlock()

	s.connectingMu.Lock()
	s.connecting = true
	s.connectingMu.Unlock()

	s.logger.Infof("connecting to %s", s.config.SerialDevice)

	for {
		sc := &serial.Config{Name: s.config.SerialDevice, Baud: s.config.Baud}
		s.logger.Debugf("attempting to open serial port %s at %d baud", s.config.SerialDevice, s.config.Baud)

		rwc, err := serial.OpenPort(sc)
		if err != nil {
			s.logger.Errorf("failed to open serial port %s: %v", s.config.SerialDevice, err)
			s.logger.Error("sleeping 30 seconds and trying again")

			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				s.connectingMu.Lock()
				s.connecting = false
				s.connectingMu.Unlock()
				return
			case <-time.After(serialRetryDelay):
			}
		} else {
			s.rwc = rwc
			s.setConnected(true)
			return
		}
	}
}

func (s *Station) connectToNetworkStation() {
	address := net.JoinHostPort(s.config.Hostname, s.config.Port)

	s.connectingMu.RLock()
	if s.connecting {
		s.connectingMu.RUnlock()
		s.logger.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}
	s.connectingMu.RUnlock()

	s.connectingMu.Lock()
	s.connecting = true
	s.connectingMu.Unlock()

	s.logger.Infof("connecting to %s", address)

	for {
		netConn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			s.logger.Errorf("could not connect to %s: %v", address, err)
			s.logger.Error("sleeping 5 seconds and trying again")

			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				s.connectingMu.Lock()
				s.connecting = false
				s.connectingMu.Unlock()
				return
			case <-time.After(retryDelay):
			}
		} else {
			netConn.SetReadDeadline(time.Now().Add(networkTimeout))
			s.netConn = netConn
			s.rwc = io.ReadWriteCloser(netConn)
			s.setConnected(true)
			return
		}
	}
}

func (s *Station) setConnected(connected bool) {
	s.connectedMu.Lock()
	s.connected = connected
	s.connectedMu.Unlock()

	s.connectingMu.Lock()
	s.connecting = false
	s.connectingMu.Unlock()
}
