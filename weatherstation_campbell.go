package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	serial "github.com/tarm/goserial"
	"go.uber.org/zap"
)

// CampbellScientificWeatherStation holds our connection along with some mutexes for operation
type CampbellScientificWeatherStation struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	netConn            net.Conn
	rwc                io.ReadWriteCloser
	Config             DeviceConfig
	ReadingDistributor chan Reading
	Logger             *zap.SugaredLogger
	connecting         bool
	connectingMu       sync.RWMutex
	connected          bool
	connectedMu        sync.RWMutex
}

// CampbellPacket describes the structured data outputted by the data logger
type CampbellPacket struct {
	StationBatteryVoltage float32 `json:"batt_volt,omitempty"`
	OutTemp               float32 `json:"airtemp_f,omitempty"`
	OutHumidity           float32 `json:"rh,omitempty"`
	Barometer             float32 `json:"baro,omitempty"`
	ExtraTemp1            float32 `json:"baro_temp_f,omitempty"`
	SolarWatts            float32 `json:"solar_w,omitempty"`
	SolarJoules           float32 `json:"solar_j,omitempty"`
	RainIncremental       float32 `json:"rain_in,omitempty"`
	WindSpeed             float32 `json:"wind_s,omitempty"`
	WindDir               uint16  `json:"wind_d,omitempty"`
}

func NewCampbellScientificWeatherStation(ctx context.Context, wg *sync.WaitGroup, c DeviceConfig, distributor chan Reading, logger *zap.SugaredLogger) (*CampbellScientificWeatherStation, error) {
	d := CampbellScientificWeatherStation{
		ctx:                ctx,
		wg:                 wg,
		Config:             c,
		ReadingDistributor: distributor,
		Logger:             logger,
	}

	if c.SerialDevice == "" && (c.Hostname == "" || c.Port == "") {
		return &d, fmt.Errorf("must define either a serial device or hostname+port")
	}

	if c.SerialDevice != "" {
		log.Info("Configuring Campbell Scientific station via serial port...")
	}

	if c.Hostname != "" && c.Port == "" {
		log.Info("Configuring Campbell Scientific station via TCP/IP")
	}

	return &d, nil
}

func (w *CampbellScientificWeatherStation) StationName() string {
	return w.Config.Name
}

// StartWeatherStation wakes the station and launches the station-polling goroutine
func (w *CampbellScientificWeatherStation) StartWeatherStation() error {
	log.Infof("Starting Campbell Scientific weather station [%v]...", w.Config.Name)

	// Wake the console
	w.ConnectToStation()

	w.wg.Add(1)
	go w.GetCampbellScientificPackets()

	return nil

}

// ConnectToStation sends a series of carriage returns in an attempt to awaken the station
func (w *CampbellScientificWeatherStation) ConnectToStation() {
	var alive bool
	var err error
	w.Connect()

	for !alive {
		log.Infof("Waiting for first packet from station [%v]", w.Config.Name)
		dec := json.NewDecoder(w.rwc)
		packet := new(CampbellPacket)
		err = dec.Decode(&packet)
		if err != nil {
			log.Info("error decoding JSON from station:", err)
			log.Info("sleeping 500ms and trying again")
			time.Sleep(500 * time.Millisecond)
		} else {
			log.Infof("Station [%v] is alive", w.Config.Name)
			alive = true
			return
		}
	}
}

// GetCampbellScientificPackets runs the ParseCampbellScientificPackets function,
// reconnecting if there is an error.
func (w *CampbellScientificWeatherStation) GetCampbellScientificPackets() {
	defer w.wg.Done()
	log.Info("starting Campbell Scientific packet getter")
	for {
		select {
		case <-w.ctx.Done():
			log.Info("cancellation request recieved.  Cancelling ParseCampbellPackets()")
			return
		default:
			err := w.ParseCampbellScientificPackets()
			if err != nil {
				w.Logger.Error(err)
				w.rwc.Close()
				if len(w.Config.Hostname) > 0 {
					w.netConn.Close()
				}
				w.Logger.Info("attempting to reconnect...")
				w.Connect()
			} else {
				return
			}
		}
	}
}

// ParseCampbellPackets parses JSON packets from the station, converts them to Readings,
// and sends them to the ReadingDistributor
func (w *CampbellScientificWeatherStation) ParseCampbellScientificPackets() error {
	var cp CampbellPacket

	scanner := bufio.NewScanner(w.rwc)

	for scanner.Scan() {
		select {
		case <-w.ctx.Done():
			log.Info("cancellation request recieved.  Cancelling ParseCampbellPackets()")
			return nil
		default:
			err := json.Unmarshal(scanner.Bytes(), &cp)
			if err != nil {
				return fmt.Errorf("error unmarshalling JSON: %v", err)
			}

			r := Reading{
				Timestamp:             time.Now(),
				StationName:           w.Config.Name,
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
			}

			wc, useWc := calcWindChill(r.OutTemp, r.WindSpeed)
			if useWc {
				r.Windchill = wc
			}

			hi, useHi := calcHeatIndex(r.OutTemp, r.OutHumidity)
			if useHi {
				r.HeatIndex = hi
			}

			// Send the reading to the distributor
			w.ReadingDistributor <- r
		}
	}

	return fmt.Errorf("scanning aborted due to error or EOF")
}

// Connect connects to a Davis station over TCP/IP
func (w *CampbellScientificWeatherStation) Connect() {
	if len(w.Config.SerialDevice) > 0 {
		w.connectToSerialStation()
	} else if (len(w.Config.Hostname) > 0) && (len(w.Config.Port) > 0) {
		w.connectToNetworkStation()
	} else {
		w.Logger.Fatal("must provide either network hostname+port or serial device in config")
	}
}

// Connect connects to a Davis station over TCP/IP
func (w *CampbellScientificWeatherStation) connectToSerialStation() {
	var err error

	w.connectingMu.RLock()

	if w.connecting {
		w.connectingMu.RUnlock()
		w.Logger.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}

	// A connection attempt is not in progress so we'll start a new one
	w.connectingMu.RUnlock()
	w.connectingMu.Lock()
	w.connecting = true
	w.connectingMu.Unlock()

	w.Logger.Infof("connecting to %v ...", w.Config.SerialDevice)

	for {
		sc := &serial.Config{Name: w.Config.SerialDevice, Baud: 19200}
		w.rwc, err = serial.OpenPort(sc)

		if err != nil {
			// There is a known problem where some shitty USB <-> serial adapters will drop out and Linux
			// will reattach them under a new device.  This code doesn't handle this situation currently
			// but it would be a nice enhancement in the future.
			w.Logger.Error("sleeping 30 seconds and trying again")
			time.Sleep(30 * time.Second)
		} else {
			// We're connected now so we set connected to true and connecting to false
			w.connectedMu.Lock()
			defer w.connectedMu.Unlock()
			w.connected = true
			w.connectingMu.Lock()
			defer w.connectingMu.Unlock()
			w.connecting = false

			return
		}
	}

}

// Connect connects to a Davis station over TCP/IP
func (w *CampbellScientificWeatherStation) connectToNetworkStation() {
	var err error

	console := fmt.Sprint(w.Config.Hostname, ":", w.Config.Port)

	w.connectingMu.RLock()

	if w.connecting {
		w.connectingMu.RUnlock()
		log.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}

	// A connection attempt is not in progress so we'll start a new one
	w.connectingMu.RUnlock()
	w.connectingMu.Lock()
	w.connecting = true
	w.connectingMu.Unlock()

	log.Info("connecting to:", console)

	for {
		w.netConn, err = net.DialTimeout("tcp", console, 10*time.Second)
		w.netConn.SetReadDeadline(time.Now().Add(time.Second * 30))

		if err != nil {
			log.Errorf("could not connect to %v: %v", console, err)
			log.Error("sleeping 5 seconds and trying again.")
			time.Sleep(5 * time.Second)
		} else {
			// We're connected now so we set connected to true and connecting to false
			w.connectedMu.Lock()
			defer w.connectedMu.Unlock()
			w.connected = true
			w.connectingMu.Lock()
			defer w.connectingMu.Unlock()
			w.connecting = false

			// Create an io.ReadWriteCloser for our connection
			w.rwc = io.ReadWriteCloser(w.netConn)
			return
		}
	}

}
