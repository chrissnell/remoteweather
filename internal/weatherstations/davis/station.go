// Package davis provides Davis weather station support with LOOP packet communication.
package davis

// Device-specific code was ported to Go from Tom Keffer's weewx by Chris Snell
// https://github.com/weewx/weewx/blob/master/bin/weewx/drivers/vantage.py

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/crc16"
	serial "github.com/tarm/goserial"
	"go.uber.org/zap"
)

const (
	// Define some constants that are used frequently in the Davis API

	// ACK - Acknowledge packet
	ACK = "\x06"
	// RESEND - Resend packet
	RESEND = "\x15"

	maxTries = 3
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

// LoopPacket is the data returned from the Davis API "LOOP" operation
type LoopPacket struct {
	Loop               [3]byte
	LoopType           int8
	PacketType         uint8
	NextRecord         uint16
	Barometer          uint16
	InTemp             int16
	InHumidity         uint8
	OutTemp            int16
	WindSpeed          uint8
	WindSpeed10        uint8
	WindDir            uint16
	ExtraTemp1         uint8
	ExtraTemp2         uint8
	ExtraTemp3         uint8
	ExtraTemp4         uint8
	ExtraTemp5         uint8
	ExtraTemp6         uint8
	ExtraTemp7         uint8
	SoilTemp1          uint8
	SoilTemp2          uint8
	SoilTemp3          uint8
	SoilTemp4          uint8
	LeafTemp1          uint8
	LeafTemp2          uint8
	LeafTemp3          uint8
	LeafTemp4          uint8
	OutHumidity        uint8
	ExtraHumidity1     uint8
	ExtraHumidity2     uint8
	ExtraHumidity3     uint8
	ExtraHumidity4     uint8
	ExtraHumidity5     uint8
	ExtraHumidity6     uint8
	ExtraHumidity7     uint8
	RainRate           uint16
	UV                 uint8
	Radiation          uint16
	StormRain          uint16
	StormStart         uint16
	DayRain            uint16
	MonthRain          uint16
	YearRain           uint16
	DayET              uint16
	MonthET            uint16
	YearET             uint16
	SoilMoisture1      uint8
	SoilMoisture2      uint8
	SoilMoisture3      uint8
	SoilMoisture4      uint8
	LeafWetness1       uint8
	LeafWetness2       uint8
	LeafWetness3       uint8
	LeafWetness4       uint8
	InsideAlarm        uint8
	RainAlarm          uint8
	OutsideAlarm1      uint8
	OutsideAlarm2      uint8
	ExtraAlarm1        uint8
	ExtraAlarm2        uint8
	ExtraAlarm3        uint8
	ExtraAlarm4        uint8
	ExtraAlarm5        uint8
	ExtraAlarm6        uint8
	ExtraAlarm7        uint8
	ExtraAlarm8        uint8
	SoilLeafAlarm1     uint8
	SoilLeafAlarm2     uint8
	SoilLeafAlarm3     uint8
	SoilLeafAlarm4     uint8
	TxBatteryStatus    uint8
	ConsBatteryVoltage uint16
	ForecastIcon       uint8
	ForecastRule       uint8
	Sunrise            uint16
	Sunset             uint16
}

// LoopPacketWithTrend is an alternative loop packet type with 3-hour barometer trend
type LoopPacketWithTrend struct {
	LoopPacket
	Trend int8
}

func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	if err := weatherstations.ValidateSerialOrNetwork(*deviceConfig); err != nil {
		logger.Fatal(err)
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

// Capabilities returns the measurement capabilities of this station.
// Davis stations provide standard weather measurements.
func (s *Station) Capabilities() weatherstations.Capabilities {
	return weatherstations.Capabilities(weatherstations.Weather)
}

func (s *Station) StartWeatherStation() error {
	s.logger.Infof("Starting Davis weather station [%s]", s.config.Name)

	s.Connect()

	if err := s.WakeStation(); err != nil {
		s.logger.Warnf("initial wake attempt failed, will retry in loop: %v", err)
	}

	s.wg.Add(1)
	go s.GetLoopPackets()

	return nil
}

func (s *Station) StopWeatherStation() error {
	s.logger.Infof("Stopping Davis weather station [%s]", s.config.Name)
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

// GetLoopPackets gets 20 LOOP packets at a time. The Davis API supports more
// but tends to be flaky and 20 is a safe bet for each LOOP run
func (s *Station) GetLoopPackets() {
	defer s.wg.Done()
	log.Info("starting Davis LOOP packet getter")
	for {
		select {
		case <-s.ctx.Done():
			log.Info("cancellation request received. Cancelling GetLoopPackets()")
			return
		default:
			err := s.GetDavisLoopPackets(20)
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

// Connect connects to a Davis station over TCP/IP or serial
func (s *Station) Connect() {
	if len(s.config.SerialDevice) > 0 {
		s.connectToSerialStation()
	} else if (len(s.config.Hostname) > 0) && (len(s.config.Port) > 0) {
		s.connectToNetworkStation()
	} else {
		s.logger.Fatal("must provide either network hostname+port or serial device in config")
	}
}

// connectToSerialStation connects to a Davis station over serial port
func (s *Station) connectToSerialStation() {
	var err error

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

	s.logger.Infof("connecting to %v ...", s.config.SerialDevice)

	for {
		sc := &serial.Config{Name: s.config.SerialDevice, Baud: s.config.Baud}
		s.logger.Debugf("attempting to open serial port %s at %d baud", s.config.SerialDevice, s.config.Baud)
		s.rwc, err = serial.OpenPort(sc)

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
			case <-time.After(30 * time.Second):
				// Continue to next iteration
			}
		} else {
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

// connectToNetworkStation connects to a Davis station over TCP/IP
func (s *Station) connectToNetworkStation() {
	var err error

	console := fmt.Sprint(s.config.Hostname, ":", s.config.Port)

	s.connectingMu.RLock()
	if s.connecting {
		s.connectingMu.RUnlock()
		log.Info("skipping reconnect since a connection attempt is already in progress")
		return
	}

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
			s.netConn.SetReadDeadline(time.Now().Add(time.Second * 30))

			s.connectedMu.Lock()
			defer s.connectedMu.Unlock()
			s.connected = true
			s.connectingMu.Lock()
			defer s.connectingMu.Unlock()
			s.connecting = false

			s.rwc = io.ReadWriteCloser(s.netConn)
			return
		}
	}
}

// Write writes data to the connection and logs it
func (s *Station) Write(p []byte) (nn int, err error) {
	if s.logger != nil && len(p) > 0 {
		s.logger.Debugf("writing to Davis station: %s", hex.EncodeToString(p))
	}

	if s.rwc == nil {
		return 0, fmt.Errorf("connection not established")
	}

	nn, err = s.rwc.Write(p)
	if err != nil {
		s.logger.Errorf("error writing to Davis station: %v", err)
	}

	return nn, err
}

// WakeStation sends wake-up commands to the Davis station
func (s *Station) WakeStation() error {
	s.logger.Debug("waking Davis station...")

	// Send a series of line feeds to wake up the console
	for i := 0; i < 3; i++ {
		_, err := s.Write([]byte("\n"))
		if err != nil {
			return fmt.Errorf("error sending wake command: %w", err)
		}
		time.Sleep(1200 * time.Millisecond)
	}

	// Look for the "\n\r" response that indicates the console is awake
	b := make([]byte, 1024)
	_, err := s.rwc.Read(b)
	if err != nil {
		return fmt.Errorf("error reading wake response: %w", err)
	}

	s.logger.Debug("Davis station is awake")
	return nil
}

// sendData sends data to the Davis station without CRC
func (s *Station) sendData(d []byte) error {
	resp := make([]byte, 1)

	// Write the data
	s.Write(d)

	_, err := s.rwc.Read(resp)
	if err != nil {
		s.logger.Info("error reading response:", err)
		return err
	}

	// See if it was ACKed
	if resp[0] != 0x06 {
		return fmt.Errorf("no <ACK> received from console")
	}
	return nil
}

// GetDavisLoopPackets gets n LOOP packets from the Davis station
func (s *Station) GetDavisLoopPackets(n int) error {
	var err error

	for tries := 1; tries <= maxTries; tries++ {
		if tries == maxTries {
			return fmt.Errorf("tried to initiate LOOP %v times, unsucessfully", tries)
		}

		s.logger.Debugf("initiating LOOP mode for %d packets", n)

		// Send a LOOP request up to (maxTries) times
		err = s.sendData([]byte(fmt.Sprintf("LOOP %v\n", n)))
		if err != nil {
			s.logger.Error(err)
			tries++
		} else {
			break
		}
	}

	// Wait for 1 second but respect context cancellation
	select {
	case <-time.After(1 * time.Second):
	case <-s.ctx.Done():
		s.logger.Info("cancellation request received. Cancelling GetDavisLoopPackets() during initial wait")
		return nil
	}

	tries := 1

	scanner := bufio.NewScanner(s.rwc)
	scanner.Split(s.scanPackets)

	buf := make([]byte, 99)
	scanner.Buffer(buf, 99)

	for l := 0; l < n; l++ {

		// Wait for 1 second but respect context cancellation
		select {
		case <-time.After(1 * time.Second):
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received. Cancelling GetDavisLoopPackets() during loop wait")
			return nil
		}

		if tries > maxTries {
			s.logger.Error("max retries exceeded while getting loop data")
			return nil
		}

		if len(s.config.Hostname) > 0 {
			err = s.netConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				s.logger.Error("error setting read deadline:", err)
			}
		}

		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received. Cancelling GetDavisLoopPackets()")
			return nil
		default:
			scanner.Scan()

			if err = scanner.Err(); err != nil {
				return fmt.Errorf("error while reading from console, LOOP %v: %v", l, err)
			}

			buf = scanner.Bytes()

			s.logger.Debugf("read packet: %s", hex.Dump(buf))

			if len(buf) < 99 {
				s.logger.Infof("packet too short, rejecting: length=%d", len(buf))
				tries++
				continue
			}

			// Verify CRC16 checksum
			if crc16.Crc16(buf) != 0 {
				s.logger.Errorf("LOOP %v CRC error (try #%v)", l, tries)
				tries++
				continue
			}

			unpacked, err := s.unpackLoopPacket(buf)
			if err != nil {
				tries++
				s.logger.Errorf("error unpacking loop packet: %v (try #%v)", err, tries)
				continue
			}

			tries = 1

			r := s.convertLoopPacket(unpacked)

			// Set the timestamp on our reading to the current system time
			r.Timestamp = time.Now()
			r.StationName = s.config.Name
			r.StationType = "davis"

			s.logger.Debugf("Packet received: %+v", r)

			s.logger.Debugf("Davis [%s] sending reading: temp=%.1f°F, humidity=%.1f%%, wind=%.1f mph @ %d°, pressure=%.2f\"",
				s.config.Name, r.OutTemp, r.OutHumidity, r.WindSpeed, int(r.WindDir), r.Barometer)
			s.ReadingDistributor <- r
		}
	}
	return nil
}

// scanPackets is a custom scanner function for LOOP packets
func (s *Station) scanPackets(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Davis LOOP packets are exactly 99 bytes with no terminators
	// First 3 bytes should be "LOO" (0x4C 0x4F 0x4F)
	if len(data) >= 99 {
		// Check if we have a valid LOOP packet header
		if len(data) >= 3 && data[0] == 0x4C && data[1] == 0x4F && data[2] == 0x4F {
			s.logger.Debugf("scanPackets found complete LOOP packet (99 bytes)")
			return 99, data[:99], nil
		}
	}

	// Look for LOOP packet header in the buffer
	for i := 0; i <= len(data)-3; i++ {
		if data[i] == 0x4C && data[i+1] == 0x4F && data[i+2] == 0x4F {
			// Found LOOP header, check if we have enough bytes for a complete packet
			if len(data) >= i+99 {
				s.logger.Debugf("scanPackets found LOOP packet at offset %d", i)
				return i + 99, data[i : i+99], nil
			}
			// Not enough data yet, request more
			return 0, nil, nil
		}
	}

	if atEOF && len(data) > 0 {
		return len(data), data[0:], io.EOF
	}

	// Request more data
	return 0, nil, nil
}

// unpackLoopPacket unpacks a binary LOOP packet into a struct
func (s *Station) unpackLoopPacket(p []byte) (*LoopPacketWithTrend, error) {
	var trend int8
	var isFlavorA bool
	var lpwt *LoopPacketWithTrend

	lp := new(LoopPacket)

	// OK, this is super goofy: the loop packets come in two flavors: A and B.
	// Flavor A will always have the character 'P' (ASCII 80) as the fourth byte of the packet
	// Flavor B will have the 3-hour barometer trend in this position instead

	// So, first we create a new Reader from the packet...
	r := bytes.NewReader(p)

	// Then we make a 1-byte slice
	peek := make([]byte, 1)

	// And we skip the first three bytes of the packet and read the fourth byte into peek
	_, err := r.ReadAt(peek, 3)
	if err != nil {
		return nil, err
	}

	// Now we compare the fourth byte (peek) of the packet to see if it's set to 'P'
	if bytes.Equal(peek, []byte{80}) {
		// It's set to 'P', so we set isFlavorA to true.  Following the weewx convention, we'll later set PacketType
		// to 'A' (ASCII 65) to signify a Flavor-A packet.
		isFlavorA = true
	} else {
		// The fourth byte was not 'P' so we now know that it's our 3-hour barometer trend.   Create a Reader
		// from this byte, decode it into an int8, then save the byte value to trend for later assignment in
		// our object.
		peekr := bytes.NewReader(peek)
		err = binary.Read(peekr, binary.LittleEndian, &trend)
		if err != nil {
			return nil, err
		}
	}

	// Now we read in the loop packet into our LoopPacket struct
	err = binary.Read(r, binary.LittleEndian, lp)
	if err != nil {
		return nil, err
	}

	if isFlavorA {
		// For Flavor-A packets, we build a LoopPacketWithTrend but set trend to 0 and PacketType to 'A'
		lp.PacketType = 65
		lpwt = &LoopPacketWithTrend{*lp, 0}
	} else {
		// For Flavor-B packets, we build a LoopPacketWithTrend and set trend to the value we extracted
		lpwt = &LoopPacketWithTrend{*lp, trend}
	}

	return lpwt, nil
}

// convertLoopPacket converts a Davis LOOP packet to a standard Reading
func (s *Station) convertLoopPacket(lp *LoopPacketWithTrend) types.Reading {
	timestamp := time.Now()

	return types.Reading{
		Timestamp:             timestamp,
		StationName:           s.config.Name,
		StationType:           "davis",
		Barometer:             s.convVal1000Zero(lp.Barometer),
		InTemp:                s.convBigVal10(lp.InTemp),
		InHumidity:            s.convLittleVal(lp.InHumidity),
		OutTemp:               s.convBigVal10(lp.OutTemp),
		WindSpeed:             s.convLittleVal(lp.WindSpeed),
		WindSpeed10:           s.convLittleVal(lp.WindSpeed10),
		WindDir:               s.correctWindDirection(lp.WindDir),
		ExtraTemp1:            s.convLittleTemp(lp.ExtraTemp1),
		ExtraTemp2:            s.convLittleTemp(lp.ExtraTemp2),
		ExtraTemp3:            s.convLittleTemp(lp.ExtraTemp3),
		ExtraTemp4:            s.convLittleTemp(lp.ExtraTemp4),
		ExtraTemp5:            s.convLittleTemp(lp.ExtraTemp5),
		ExtraTemp6:            s.convLittleTemp(lp.ExtraTemp6),
		ExtraTemp7:            s.convLittleTemp(lp.ExtraTemp7),
		SoilTemp1:             s.convLittleTemp(lp.SoilTemp1),
		SoilTemp2:             s.convLittleTemp(lp.SoilTemp2),
		SoilTemp3:             s.convLittleTemp(lp.SoilTemp3),
		SoilTemp4:             s.convLittleTemp(lp.SoilTemp4),
		LeafTemp1:             s.convLittleTemp(lp.LeafTemp1),
		LeafTemp2:             s.convLittleTemp(lp.LeafTemp2),
		LeafTemp3:             s.convLittleTemp(lp.LeafTemp3),
		LeafTemp4:             s.convLittleTemp(lp.LeafTemp4),
		OutHumidity:           s.convLittleVal(lp.OutHumidity),
		ExtraHumidity1:        s.convLittleVal(lp.ExtraHumidity1),
		ExtraHumidity2:        s.convLittleVal(lp.ExtraHumidity2),
		ExtraHumidity3:        s.convLittleVal(lp.ExtraHumidity3),
		ExtraHumidity4:        s.convLittleVal(lp.ExtraHumidity4),
		ExtraHumidity5:        s.convLittleVal(lp.ExtraHumidity5),
		ExtraHumidity6:        s.convLittleVal(lp.ExtraHumidity6),
		ExtraHumidity7:        s.convLittleVal(lp.ExtraHumidity7),
		RainRate:              s.convBigVal100(lp.RainRate),
		UV:                    s.convLittleVal10(lp.UV),
		SolarWatts:            s.convBigVal(lp.Radiation),
		StormRain:             s.convVal100(lp.StormRain),
		DayRain:               s.convVal100(lp.DayRain),
		MonthRain:             s.convVal100(lp.MonthRain),
		YearRain:              s.convVal100(lp.YearRain),
		DayET:                 s.convVal1000(lp.DayET),
		MonthET:               s.convVal100(lp.MonthET),
		YearET:                s.convVal100(lp.YearET),
		SoilMoisture1:         s.convLittleVal(lp.SoilMoisture1),
		SoilMoisture2:         s.convLittleVal(lp.SoilMoisture2),
		SoilMoisture3:         s.convLittleVal(lp.SoilMoisture3),
		SoilMoisture4:         s.convLittleVal(lp.SoilMoisture4),
		LeafWetness1:          s.convLittleVal(lp.LeafWetness1),
		LeafWetness2:          s.convLittleVal(lp.LeafWetness2),
		LeafWetness3:          s.convLittleVal(lp.LeafWetness3),
		LeafWetness4:          s.convLittleVal(lp.LeafWetness4),
		InsideAlarm:           lp.InsideAlarm,
		RainAlarm:             lp.RainAlarm,
		OutsideAlarm1:         lp.OutsideAlarm1,
		OutsideAlarm2:         lp.OutsideAlarm2,
		ExtraAlarm1:           lp.ExtraAlarm1,
		ExtraAlarm2:           lp.ExtraAlarm2,
		ExtraAlarm3:           lp.ExtraAlarm3,
		ExtraAlarm4:           lp.ExtraAlarm4,
		ExtraAlarm5:           lp.ExtraAlarm5,
		ExtraAlarm6:           lp.ExtraAlarm6,
		ExtraAlarm7:           lp.ExtraAlarm7,
		ExtraAlarm8:           lp.ExtraAlarm8,
		SoilLeafAlarm1:        lp.SoilLeafAlarm1,
		SoilLeafAlarm2:        lp.SoilLeafAlarm2,
		SoilLeafAlarm3:        lp.SoilLeafAlarm3,
		SoilLeafAlarm4:        lp.SoilLeafAlarm4,
		TxBatteryStatus:       lp.TxBatteryStatus,
		StationBatteryVoltage: s.convConsBatteryVoltage(lp.ConsBatteryVoltage),
		ForecastIcon:          lp.ForecastIcon,
		ForecastRule:          lp.ForecastRule,
		WindChill:             weatherstations.CalculateWindChill(s.convBigVal10(lp.OutTemp), s.convLittleVal(lp.WindSpeed)),
		HeatIndex:             weatherstations.CalculateHeatIndex(s.convBigVal10(lp.OutTemp), s.convLittleVal(lp.OutHumidity)),
	}
}

// Conversion helper functions
func (s *Station) convVal100(v uint16) float32 {
	return float32(v) / 100.0
}

func (s *Station) convVal1000(v uint16) float32 {
	return float32(v) / 1000.0
}

func (s *Station) convBigVal(v uint16) float32 {
	if v == 0x7FFF {
		return 0
	}
	return float32(v)
}

// correctWindDirection applies the configured wind direction correction
func (s *Station) correctWindDirection(windDir uint16) float32 {
	if windDir == 0x7FFF {
		return 0
	}

	corrected := int16(windDir)

	if s.config.WindDirCorrection != 0 {
		s.logger.Debugf("Correcting wind direction by %v degrees", s.config.WindDirCorrection)
		corrected += s.config.WindDirCorrection

		// Normalize to 0-359 range
		for corrected >= 360 {
			corrected -= 360
		}
		for corrected < 0 {
			corrected += 360
		}
	}

	return float32(corrected)
}

func (s *Station) convBigVal10(v int16) float32 {
	if v == -32768 {
		return 0
	}
	return float32(v) / 10.0
}

func (s *Station) convBigVal100(v uint16) float32 {
	if v == 0xFFFF {
		return 0
	}
	return float32(v) / 100.0
}

func (s *Station) convLittleVal(v uint8) float32 {
	if v == 0xFF {
		return 0
	}
	return float32(v)
}

func (s *Station) convLittleTemp(v uint8) float32 {
	if v == 0xFF {
		return 0
	}
	return float32(v) - 90.0
}

func (s *Station) convConsBatteryVoltage(v uint16) float32 {
	return (float32(v) * 300.0) / 512.0 / 100.0
}

func (s *Station) convVal1000Zero(v uint16) float32 {
	if v == 0x7FFF {
		return 0
	}
	return float32(v) / 1000.0
}

func (s *Station) convLittleVal10(v uint8) float32 {
	if v == 0xFF {
		return 0
	}
	return float32(v) / 10.0
}
