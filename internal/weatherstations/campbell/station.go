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

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/solar"
	serial "github.com/tarm/goserial"
	"go.uber.org/zap"
)

// Station implements a Campbell Scientific weather station
type Station struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	netConn            net.Conn
	rwc                io.ReadWriteCloser
	config             config.DeviceData
	configProvider     config.ConfigProvider
	deviceName         string
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
	connecting         bool
	connectingMu       sync.RWMutex
	connected          bool
	connectedMu        sync.RWMutex
}

// Packet describes the structured data outputted by the Campbell Scientific data logger
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

// NewStation creates a new Campbell Scientific weather station
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
		logger.Fatalf("Campbell Scientific station [%s] failed to load config: %v", deviceName, err)
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
		logger.Fatalf("Campbell Scientific station [%s] device not found in configuration", deviceName)
	}

	// Use the device configuration directly
	station.config = *deviceConfig

	if station.config.SerialDevice == "" && (station.config.Hostname == "" || station.config.Port == "") {
		logger.Fatalf("Campbell Scientific station [%s] must define either a serial device or hostname+port", station.config.Name)
	}

	if station.config.SerialDevice != "" {
		log.Info("Configuring Campbell Scientific station via serial port...")
	}

	if station.config.Hostname != "" && station.config.Port != "" {
		log.Info("Configuring Campbell Scientific station via TCP/IP")
	}

	// Use 19200 baud by default, applicable for USB connection. RS-232 should be set in the config to 115200
	if station.config.Baud == 0 {
		station.config.Baud = 19200
	}

	return station
}

func (s *Station) StationName() string {
	return s.config.Name
}

// StartWeatherStation wakes the station and launches the station-polling goroutine
func (s *Station) StartWeatherStation() error {
	log.Infof("Starting Campbell Scientific weather station [%v]...", s.config.Name)

	// Wake the console
	s.ConnectToStation()

	s.wg.Add(1)
	go s.GetCampbellScientificPackets()

	return nil
}

// ConnectToStation establishes initial connection and waits for first packet
func (s *Station) ConnectToStation() {
	var alive bool
	var err error
	s.Connect()

	for !alive {
		// Check for cancellation before each iteration
		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received while waiting for first packet")
			return
		default:
		}

		log.Infof("Waiting for first packet from station [%v]", s.config.Name)

		// Check if connection is still valid
		if s.rwc == nil {
			s.logger.Info("connection is nil, attempting to reconnect")
			s.Connect()
			continue
		}

		dec := json.NewDecoder(s.rwc)
		packet := new(Packet)
		err = dec.Decode(&packet)
		if err != nil {
			log.Info("error decoding JSON from station:", err)
			log.Info("sleeping 500ms and trying again")

			// Use a select to respect cancellation during sleep
			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				return
			case <-time.After(500 * time.Millisecond):
				// Continue to next iteration
			}
		} else {
			log.Infof("Station [%v] is alive", s.config.Name)
			alive = true
			return
		}
	}
}

// GetCampbellScientificPackets runs the ParseCampbellScientificPackets function,
// reconnecting if there is an error.
func (s *Station) GetCampbellScientificPackets() {
	defer s.wg.Done()
	log.Info("starting Campbell Scientific packet getter")
	for {
		select {
		case <-s.ctx.Done():
			log.Info("cancellation request received. Cancelling ParseCampbellPackets()")
			return
		default:
			err := s.ParseCampbellScientificPackets()
			if err != nil {
				s.logger.Error(err)
				s.rwc.Close()
				if len(s.config.Hostname) > 0 {
					s.netConn.Close()
				}
				s.logger.Info("attempting to reconnect...")
				s.Connect()
			} else {
				return
			}
		}
	}
}

// ParseCampbellScientificPackets parses JSON packets from the station, converts them to Readings,
// and sends them to the ReadingDistributor
func (s *Station) ParseCampbellScientificPackets() error {
	var cp Packet

	scanner := bufio.NewScanner(s.rwc)

	for scanner.Scan() {
		// Update read deadline for network connections to prevent timeout
		if s.netConn != nil {
			s.netConn.SetReadDeadline(time.Now().Add(time.Second * 30))
		}
		select {
		case <-s.ctx.Done():
			log.Info("cancellation request received. Cancelling ParseCampbellPackets()")
			return nil
		default:
			err := json.Unmarshal(scanner.Bytes(), &cp)
			if err != nil {
				return fmt.Errorf("error unmarshalling JSON: %v", err)
			}

			var uncorrectedWindDirection, correctedWindDirection int16
			uncorrectedWindDirection = int16(cp.WindDir)
			if s.config.WindDirCorrection != 0 {
				log.Debugf("correcting wind direction by %v", s.config.WindDirCorrection)
				correctedWindDirection = uncorrectedWindDirection + s.config.WindDirCorrection
				if correctedWindDirection >= 360 {
					correctedWindDirection = correctedWindDirection - 360
				} else if correctedWindDirection < 0 {
					correctedWindDirection = correctedWindDirection + 360
				}
				cp.WindDir = uint16(correctedWindDirection)
			}

			timestamp := time.Now()

			var potentialSolarWatts float64
			if s.config.Solar.Latitude != 0 && s.config.Solar.Longitude != 0 {
				// Calculate potential solar watts for this location and time
				potentialSolarWatts = solar.CalculateClearSkySolarRadiationASCE(timestamp, s.config.Solar.Latitude, s.config.Solar.Longitude, s.config.Solar.Altitude, float64(cp.OutTemp), float64(cp.OutHumidity))
				log.Debugf("solar calculation results: %+v", potentialSolarWatts)
			}

			r := types.Reading{
				Timestamp:             timestamp,
				StationName:           s.config.Name,
				StationType:           "campbell",
				StationBatteryVoltage: cp.StationBatteryVoltage,
				OutTemp:               cp.OutTemp,
				OutHumidity:           cp.OutHumidity,
				Barometer:             cp.Barometer,
				ExtraTemp1:            cp.ExtraTemp1,
				SolarWatts:            cp.SolarWatts,
				SolarJoules:           cp.SolarJoules,
				RainIncremental:       cp.RainIncremental,
				WindSpeed:             cp.WindSpeed,
				WindDir:               float32(cp.WindDir),
				WindChill:             CalculateWindChill(cp.OutTemp, cp.WindSpeed),
				HeatIndex:             CalculateHeatIndex(cp.OutTemp, cp.OutHumidity),
				PotentialSolarWatts:   float32(potentialSolarWatts),
			}

			// Send the reading to the distributor
			log.Debugf("Campbell Scientific [%s] sending reading to distributor: temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph @ %d°, pressure=%.2f\"",
				s.config.Name, r.OutTemp, r.OutHumidity, r.WindSpeed, int(r.WindDir), r.Barometer)
			s.ReadingDistributor <- r
		}
	}

	return fmt.Errorf("scanning aborted due to error or EOF")
}

// Connect connects to a Campbell Scientific station over serial or network
func (s *Station) Connect() {
	if len(s.config.SerialDevice) > 0 {
		s.connectToSerialStation()
	} else if (len(s.config.Hostname) > 0) && (len(s.config.Port) > 0) {
		s.connectToNetworkStation()
	} else {
		s.logger.Fatal("must provide either network hostname+port or serial device in config")
	}
}

// connectToSerialStation connects to a Campbell Scientific station over serial port
func (s *Station) connectToSerialStation() {
	var err error

	s.connectingMu.RLock()
	if s.connecting {
		s.connectingMu.RUnlock()
		s.logger.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}

	// A connection attempt is not in progress so we'll start a new one
	s.connectingMu.RUnlock()
	s.connectingMu.Lock()
	s.connecting = true
	s.connectingMu.Unlock()

	s.logger.Infof("connecting to %v ...", s.config.SerialDevice)

	for {
		sc := &serial.Config{Name: s.config.SerialDevice, Baud: s.config.Baud}
		s.logger.Debugf("attempting to open serial port %s at %d baud", s.config.SerialDevice, s.config.Baud)
		s.rwc, err = serial.OpenPort(sc)

		if err != nil {
			// There is a known problem where some shitty USB <-> serial adapters will drop out and Linux
			// will reattach them under a new device. This code doesn't handle this situation currently
			// but it would be a nice enhancement in the future.
			s.logger.Errorf("failed to open serial port %s: %v", s.config.SerialDevice, err)
			s.logger.Error("sleeping 30 seconds and trying again")

			// Use a select to respect cancellation during sleep
			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				s.connectingMu.Lock()
				s.connecting = false
				s.connectingMu.Unlock()
				return
			case <-time.After(30 * time.Second):
				// Continue to next iteration
			}
		} else {
			// We're connected now so we set connected to true and connecting to false
			s.connectedMu.Lock()
			defer s.connectedMu.Unlock()
			s.connected = true
			s.connectingMu.Lock()
			defer s.connectingMu.Unlock()
			s.connecting = false

			return
		}
	}
}

// connectToNetworkStation connects to a Campbell Scientific station over TCP/IP
func (s *Station) connectToNetworkStation() {
	var err error

	console := fmt.Sprint(s.config.Hostname, ":", s.config.Port)

	s.connectingMu.RLock()
	if s.connecting {
		s.connectingMu.RUnlock()
		log.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}

	// A connection attempt is not in progress so we'll start a new one
	s.connectingMu.RUnlock()
	s.connectingMu.Lock()
	s.connecting = true
	s.connectingMu.Unlock()

	log.Info("connecting to:", console)

	for {
		s.netConn, err = net.DialTimeout("tcp", console, 10*time.Second)

		if err != nil {
			log.Errorf("could not connect to %v: %v", console, err)
			log.Error("sleeping 5 seconds and trying again.")

			// Use a select to respect cancellation during sleep
			select {
			case <-s.ctx.Done():
				s.logger.Info("cancellation request received during retry wait")
				s.connectingMu.Lock()
				s.connecting = false
				s.connectingMu.Unlock()
				return
			case <-time.After(5 * time.Second):
				// Continue to next iteration
			}
		} else {
			// Set read deadline after successful connection
			s.netConn.SetReadDeadline(time.Now().Add(time.Second * 30))

			// We're connected now so we set connected to true and connecting to false
			s.connectedMu.Lock()
			defer s.connectedMu.Unlock()
			s.connected = true
			s.connectingMu.Lock()
			defer s.connectingMu.Unlock()
			s.connecting = false

			// Create an io.ReadWriteCloser for our connection
			s.rwc = io.ReadWriteCloser(s.netConn)
			return
		}
	}
}
