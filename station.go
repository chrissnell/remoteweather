package main

// Ported from Tom Keffer's weewx
// https://github.com/weewx/weewx/blob/master/bin/weewx/drivers/vantage.py

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/gopherwx/util/crc16"
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

// WeatherStation holds our connection along with some mutexes for operation
type WeatherStation struct {
	Name           string `json:"name"`
	netConn        net.Conn
	rwc            io.ReadWriteCloser
	Config         Config
	StorageManager *StorageManager
	Logger         *zap.SugaredLogger
	connecting     bool
	connectingMu   sync.RWMutex
	connected      bool
	connectedMu    sync.RWMutex
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

// Reading is a LoopPacketWithTrend that has been converted to human-readable values
// Administrative elements (e.g. LoopType) not related to weather readings have been
// left out.
type Reading struct {
	Timestamp          time.Time `gorm:"column:time"`
	StationName        string    `gorm:"column:stationname"`
	Barometer          float32   `gorm:"column:barometer"`
	InTemp             float32   `gorm:"column:intemp"`
	InHumidity         float32   `gorm:"column:inhumidity"`
	OutTemp            float32   `gorm:"column:outtemp"`
	WindSpeed          float32   `gorm:"column:windspeed"`
	WindSpeed10        float32   `gorm:"column:windspeed10"`
	WindDir            float32   `gorm:"column:winddir"`
	Windchill          float32   `gorm:"column:windchill"`
	HeatIndex          float32   `gorm:"column:heatindex"`
	ExtraTemp1         float32   `gorm:"column:extratemp1"`
	ExtraTemp2         float32   `gorm:"column:extratemp2"`
	ExtraTemp3         float32   `gorm:"column:extratemp3"`
	ExtraTemp4         float32   `gorm:"column:extratemp4"`
	ExtraTemp5         float32   `gorm:"column:extratemp5"`
	ExtraTemp6         float32   `gorm:"column:extratemp6"`
	ExtraTemp7         float32   `gorm:"column:extratemp7"`
	SoilTemp1          float32   `gorm:"column:soiltemp1"`
	SoilTemp2          float32   `gorm:"column:soiltemp2"`
	SoilTemp3          float32   `gorm:"column:soiltemp3"`
	SoilTemp4          float32   `gorm:"column:soiltemp4"`
	LeafTemp1          float32   `gorm:"column:leaftemp1"`
	LeafTemp2          float32   `gorm:"column:leaftemp2"`
	LeafTemp3          float32   `gorm:"column:leaftemp3"`
	LeafTemp4          float32   `gorm:"column:leaftemp4"`
	OutHumidity        float32   `gorm:"column:outhumidity"`
	ExtraHumidity1     float32   `gorm:"column:extrahumidity1"`
	ExtraHumidity2     float32   `gorm:"column:extrahumidity2"`
	ExtraHumidity3     float32   `gorm:"column:extrahumidity3"`
	ExtraHumidity4     float32   `gorm:"column:extrahumidity4"`
	ExtraHumidity5     float32   `gorm:"column:extrahumidity5"`
	ExtraHumidity6     float32   `gorm:"column:extrahumidity6"`
	ExtraHumidity7     float32   `gorm:"column:extrahumidity7"`
	RainRate           float32   `gorm:"column:rainrate"`
	UV                 float32   `gorm:"column:uv"`
	Radiation          float32   `gorm:"column:radiation"`
	StormRain          float32   `gorm:"column:stormrain"`
	StormStart         time.Time `gorm:"column:stormstart"`
	DayRain            float32   `gorm:"column:dayrain"`
	MonthRain          float32   `gorm:"column:monthrain"`
	YearRain           float32   `gorm:"column:yearrain"`
	DayET              float32   `gorm:"column:dayet"`
	MonthET            float32   `gorm:"column:monthet"`
	YearET             float32   `gorm:"column:yearet"`
	SoilMoisture1      float32   `gorm:"column:soilmoisture1"`
	SoilMoisture2      float32   `gorm:"column:soilmoisture2"`
	SoilMoisture3      float32   `gorm:"column:soilmoisture3"`
	SoilMoisture4      float32   `gorm:"column:soilmoisture4"`
	LeafWetness1       float32   `gorm:"column:leafwetness1"`
	LeafWetness2       float32   `gorm:"column:leafwetness2"`
	LeafWetness3       float32   `gorm:"column:leafwetness3"`
	LeafWetness4       float32   `gorm:"column:leafwetness4"`
	InsideAlarm        uint8     `gorm:"column:insidealarm"`
	RainAlarm          uint8     `gorm:"column:rainalarm"`
	OutsideAlarm1      uint8     `gorm:"column:outsidealarm1"`
	OutsideAlarm2      uint8     `gorm:"column:outsidealarm2"`
	ExtraAlarm1        uint8     `gorm:"column:extraalarm1"`
	ExtraAlarm2        uint8     `gorm:"column:extraalarm2"`
	ExtraAlarm3        uint8     `gorm:"column:extraalarm3"`
	ExtraAlarm4        uint8     `gorm:"column:extraalarm4"`
	ExtraAlarm5        uint8     `gorm:"column:extraalarm5"`
	ExtraAlarm6        uint8     `gorm:"column:extraalarm6"`
	ExtraAlarm7        uint8     `gorm:"column:extraalarm7"`
	ExtraAlarm8        uint8     `gorm:"column:extraalarm8"`
	SoilLeafAlarm1     uint8     `gorm:"column:soilleafalarm1"`
	SoilLeafAlarm2     uint8     `gorm:"column:soilleafalarm2"`
	SoilLeafAlarm3     uint8     `gorm:"column:soilleafalarm3"`
	SoilLeafAlarm4     uint8     `gorm:"column:soilleafalarm4"`
	TxBatteryStatus    uint8     `gorm:"column:txbatterystatus"`
	ConsBatteryVoltage float32   `gorm:"column:consbatteryvoltage"`
	ForecastIcon       uint8     `gorm:"column:forecasticon"`
	ForecastRule       uint8     `gorm:"column:forecastrule"`
	Sunrise            time.Time `gorm:"column:sunrise"`
	Sunset             time.Time `gorm:"column:sunset"`
}

// NewWeatherStation creates a new data model with a new DB connection and Kube API client
func NewWeatherStation(c Config, sto *StorageManager) *WeatherStation {

	ws := new(WeatherStation)

	ws.Config = c
	ws.StorageManager = sto

	return ws
}

// StartLoopPolling launches the station-polling goroutine and process packets as they're received
func (w *WeatherStation) StartLoopPolling() {
	packetChan := make(chan Reading)

	// Wake the console
	w.WakeStation()

	go w.GetLoopPackets(packetChan)
	w.ProcessLoopPackets(packetChan)
}

// ProcessLoopPackets processes received LOOP packets
func (w *WeatherStation) ProcessLoopPackets(packetChan <-chan Reading) {
	for {
		p := <-packetChan
		w.StorageManager.ReadingDistributor <- p
	}
}

// GetLoopPackets gets 20 LOOP packets at a time.  The Davis API supports more
// but tends to be flaky and 20 is a safe bet for each LOOP run
func (w *WeatherStation) GetLoopPackets(packetChan chan<- Reading) {
	for {
		err := w.GetDavisLoopPackets(20, packetChan)
		if err != nil {
			w.Logger.Error(err)
			w.rwc.Close()
			if len(w.Config.Device.Hostname) > 0 {
				w.netConn.Close()
			}
			w.Logger.Info("attempting to reconnect...")
			w.Connect()
		}
	}
}

// Connect connects to a Davis station over TCP/IP
func (w *WeatherStation) Connect() {
	if len(w.Config.Device.SerialDevice) > 0 {
		w.connectToSerialStation()
	} else if (len(w.Config.Device.Hostname) > 0) && (len(w.Config.Device.Port) > 0) {
		w.connectToNetworkStation()
	} else {
		w.Logger.Fatal("must provide either network hostname+port or serial device in config")
	}
}

// Connect connects to a Davis station over TCP/IP
func (w *WeatherStation) connectToSerialStation() {
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

	w.Logger.Infof("connecting to %v ...", w.Config.Device.SerialDevice)

	for {
		sc := &serial.Config{Name: w.Config.Device.SerialDevice, Baud: 19200}
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
func (w *WeatherStation) connectToNetworkStation() {
	var err error

	console := fmt.Sprint(w.Config.Device.Hostname, ":", w.Config.Device.Port)

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

func (w *WeatherStation) Write(p []byte) (nn int, err error) {
	for {
		nn, err = w.rwc.Write(p)
		if err != nil {
			// We must not be connected
			log.Info("error writing to console:", err)
			log.Info("attempting to reconnect...")
			w.Connect()
		} else {
			// Write was successful
			return nn, err
		}
	}
}

// WakeStation sends a series of carriage returns in an attempt to awaken the station
func (w *WeatherStation) WakeStation() {
	var alive bool
	var err error

	w.Connect()

	resp := make([]byte, 1024)

	for !alive {
		log.Info("waking up station.")

		w.rwc.Write([]byte("\n"))

		_, err = w.rwc.Read(resp)

		if err != nil {
			log.Fatal("could not read from station:", err)
		}
		// fmt.Println("This is what we got back:", resp)

		if resp[0] == 0x0a && resp[1] == 0x0d {
			log.Info("station has been awaken.")
			alive = true
			return
		}
		log.Info("sleeping 500ms and trying again...")
		time.Sleep(500 * time.Millisecond)

	}

}

func (w *WeatherStation) sendData(d []byte) error {
	resp := make([]byte, 1)

	// Write the data
	w.Write(d)

	_, err := w.rwc.Read(resp)
	if err != nil {
		log.Info("error reading response:", err)
		return err
	}

	// See if it was ACKed
	if resp[0] != 0x06 {
		return fmt.Errorf("no <ACK> recieved from console")
	}
	return nil
}

// Not currently utilized but can be used to set station clock, among other things
//lint:ignore U1000 For future use
func (w *WeatherStation) sendDataWithCRC16(d []byte) error {
	var resp []byte

	// We'll write to a Buffer and then dump the buffer to the device
	buf := new(bytes.Buffer)

	check := crc16.Crc16(d)

	// First, write the data
	_, err := buf.Write(d)
	if err != nil {
		return err
	}

	// Next, write the CRC in big-endian order
	err = binary.Write(buf, binary.BigEndian, check)
	if err != nil {
		return err
	}

	for i := 0; i <= maxTries; i++ {
		_, err := buf.WriteTo(w.rwc)
		if err != nil {
			return err
		}

		_, err = w.rwc.Read(resp)
		if err != nil {
			log.Error("error reading response:", err)
			return err
		}

		if resp[0] != ACK[0] {
			log.Error("no <ACK> was received from console")
			return nil
		}
	}

	return fmt.Errorf("i/o error writing data with CRC to device")
}

//lint:ignore U1000 For future use
func (w *WeatherStation) sendCommand(command []byte) ([]string, error) {
	var err error
	var resp []byte

	// We'll write to a Buffer and then dump the buffer to the device
	buf := new(bytes.Buffer)

	// We'll try to send it up to maxTries times before erroring out
	for i := 0; i <= maxTries; i++ {
		w.WakeStation()

		// First, write the data
		_, err = buf.Write(command)
		if err != nil {
			return nil, err
		}

		// Write the buffer to the device
		_, err = buf.WriteTo(w.rwc)
		if err != nil {
			return nil, err
		}

		// Sleep for 500ms to wait for the device to react and fill its buffer
		time.Sleep(500 * time.Millisecond)

		_, err = w.rwc.Read(resp)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(string(resp), "\n\r")

		if parts[0] == "OK" {
			return parts[1:], nil
		}
	}
	log.Error("tried three times to send command but failed.")
	return nil, err
}

//lint:ignore U1000 For future use
func (w *WeatherStation) getDataWithCRC16(numBytes int64, prompt string) ([]byte, error) {
	var err error

	buf := new(bytes.Buffer)

	if prompt != "" {
		// We'll write to a Buffer and then dump it to the device
		_, err = buf.WriteString(prompt)
		if err != nil {
			return nil, err
		}

		// Write the buffer to the device
		_, err = buf.WriteTo(w.rwc)
		if err != nil {
			return nil, err
		}

	}

	// We're going to try reading data from the device maxTries times...
	for i := 1; i <= maxTries; i++ {

		// If it's not our first attempt at reading from the console, we send a RESEND command
		// to goad the console into responding.
		if i > 1 {
			_, err = buf.Write([]byte(RESEND))
			if err != nil {
				log.Error("could not write RESEND command to buffer")
				return nil, err
			}
			// Write the buffer to the console
			_, err = buf.WriteTo(w.rwc)
			if err != nil {
				log.Error("could not write buffer to console")
				return nil, err
			}

			checkBytes := make([]byte, numBytes)

			_, err := w.rwc.Read(checkBytes)
			if err != nil {
				return nil, err
			}

			// Do a CRC16 check on data we read and return data if it passes
			if crc16.Crc16(checkBytes) == uint16(0) {
				return checkBytes, nil
			}

			// We didn't pass the CRC check so we loop again.
			log.Error("the data read did not pass the CRC16 check")
		}
	}

	// We failed at reading data from the console
	return nil, fmt.Errorf("failed to read any data from the console after %v attempts", maxTries)
}

// GetDavisLoopPackets attempts to initiate a LOOP command against the station and retrieve some packets
func (w *WeatherStation) GetDavisLoopPackets(n int, packetChan chan<- Reading) error {
	var err error

	for tries := 1; tries <= maxTries; tries++ {
		if tries == maxTries {
			return fmt.Errorf("tried to initiate LOOP %v times, unsucessfully", tries)
		}

		if *debug {
			log.Info("initiating LOOP mode for", n, "packets.")
		}

		// Send a LOOP request up to (maxTries) times
		err = w.sendData([]byte(fmt.Sprintf("LOOP %v\n", n)))
		if err != nil {
			log.Error(err)
			tries++
		} else {
			break
		}
	}

	time.Sleep(1 * time.Second)

	tries := 1

	scanner := bufio.NewScanner(w.rwc)
	scanner.Split(scanPackets)

	buf := make([]byte, 99)
	scanner.Buffer(buf, 99)

	for l := 0; l < n; l++ {

		time.Sleep(1 * time.Second)

		if tries > maxTries {
			log.Error("max retries exeeded while getting loop data")
			return nil
		}

		if len(w.Config.Device.Hostname) > 0 {
			err = w.netConn.SetReadDeadline(time.Now().Add(5 * time.Second))

			if err != nil {
				log.Error("error setting read deadline:", err)
			}

		}

		scanner.Scan()

		if err = scanner.Err(); err != nil {
			return fmt.Errorf("error while reading from console, LOOP %v: %v", l, err)
		}

		buf = scanner.Bytes()

		log.Debugw("read packet:", "packet_contents", hex.Dump(buf))

		if len(buf) < 99 {
			log.Infow("packet too short, rejecting", "packet_length", len(buf), "raw_packet", hex.Dump(buf))
			tries++
			continue
		}

		if buf[95] != 0x0A && buf[96] != 0x0D {
			log.Error("end-of-packet signature not found; rejecting.")
		} else {

			if crc16.Crc16(buf) != 0 {
				log.Errorf("LOOP %v CRC error (try #%v)", l, tries)
				tries++
				continue
			}

			unpacked, err := w.unpackLoopPacket(buf)
			if err != nil {
				tries++
				log.Errorf("error unpacking loop packet: %v (try #%v)", err, tries)
				continue
			}

			tries = 1

			r := convValues(unpacked)

			// Set the timestamp on our reading to the current system time
			r.Timestamp = time.Now()
			r.StationName = w.Config.Device.Name

			log.Debugf("packet recieved: %+v", r)

			packetChan <- r
			//loopPackets = append(loopPackets, unpacked)
		}
	}
	return nil
}

func scanPackets(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < (len(data) - 3); i++ {

		log.Debugf("scanPackets byte: %v  data: %+v\n", i, data)

		if data[i] == 0x0A && data[i+1] == 0x0D {
			return i + 4, data[:i+4], nil
		}
	}

	if atEOF && len(data) > 0 {
		return len(data), data[0:], io.EOF
	}

	// Request more data.

	return 0, nil, nil
}

func (w *WeatherStation) unpackLoopPacket(p []byte) (*LoopPacketWithTrend, error) {
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

func convValues(lp *LoopPacketWithTrend) Reading {
	r := Reading{
		Barometer:          convVal1000Zero(lp.Barometer),
		InTemp:             convBigVal10(lp.InTemp),
		InHumidity:         convLittleVal(lp.InHumidity),
		OutTemp:            convBigVal10(lp.OutTemp),
		WindSpeed:          convLittleVal(lp.WindSpeed),
		WindSpeed10:        convLittleVal(lp.WindSpeed10),
		WindDir:            convBigVal(lp.WindDir),
		ExtraTemp1:         convLittleTemp(lp.ExtraTemp1),
		ExtraTemp2:         convLittleTemp(lp.ExtraTemp2),
		ExtraTemp3:         convLittleTemp(lp.ExtraTemp3),
		ExtraTemp4:         convLittleTemp(lp.ExtraTemp4),
		ExtraTemp5:         convLittleTemp(lp.ExtraTemp5),
		ExtraTemp6:         convLittleTemp(lp.ExtraTemp6),
		ExtraTemp7:         convLittleTemp(lp.ExtraTemp7),
		SoilTemp1:          convLittleTemp(lp.SoilTemp1),
		SoilTemp2:          convLittleTemp(lp.SoilTemp2),
		SoilTemp3:          convLittleTemp(lp.SoilTemp3),
		SoilTemp4:          convLittleTemp(lp.SoilTemp4),
		LeafTemp1:          convLittleTemp(lp.LeafTemp1),
		LeafTemp2:          convLittleTemp(lp.LeafTemp2),
		LeafTemp3:          convLittleTemp(lp.LeafTemp3),
		LeafTemp4:          convLittleTemp(lp.LeafTemp4),
		OutHumidity:        convLittleVal(lp.OutHumidity),
		ExtraHumidity1:     convLittleVal(lp.ExtraHumidity1),
		ExtraHumidity2:     convLittleVal(lp.ExtraHumidity2),
		ExtraHumidity3:     convLittleVal(lp.ExtraHumidity3),
		ExtraHumidity4:     convLittleVal(lp.ExtraHumidity4),
		ExtraHumidity5:     convLittleVal(lp.ExtraHumidity5),
		ExtraHumidity6:     convLittleVal(lp.ExtraHumidity6),
		ExtraHumidity7:     convLittleVal(lp.ExtraHumidity7),
		RainRate:           convBigVal100(lp.RainRate),
		UV:                 convLittleVal10(lp.UV),
		Radiation:          convBigVal(lp.Radiation),
		StormRain:          convVal100(lp.StormRain),
		StormStart:         convLoopDate(lp.StormStart),
		DayRain:            convVal100(lp.DayRain),
		MonthRain:          convVal100(lp.MonthRain),
		YearRain:           convVal100(lp.YearRain),
		DayET:              convVal1000(lp.DayET),
		MonthET:            convVal100(lp.MonthET),
		YearET:             convVal100(lp.YearET),
		SoilMoisture1:      convLittleVal(lp.SoilMoisture1),
		SoilMoisture2:      convLittleVal(lp.SoilMoisture2),
		SoilMoisture3:      convLittleVal(lp.SoilMoisture3),
		SoilMoisture4:      convLittleVal(lp.SoilMoisture4),
		LeafWetness1:       convLittleVal(lp.LeafWetness1),
		LeafWetness2:       convLittleVal(lp.LeafWetness2),
		LeafWetness3:       convLittleVal(lp.LeafWetness3),
		LeafWetness4:       convLittleVal(lp.LeafWetness4),
		InsideAlarm:        lp.InsideAlarm,
		RainAlarm:          lp.RainAlarm,
		OutsideAlarm1:      lp.OutsideAlarm1,
		OutsideAlarm2:      lp.OutsideAlarm2,
		ExtraAlarm1:        lp.ExtraAlarm1,
		ExtraAlarm2:        lp.ExtraAlarm2,
		ExtraAlarm3:        lp.ExtraAlarm3,
		ExtraAlarm4:        lp.ExtraAlarm4,
		ExtraAlarm5:        lp.ExtraAlarm5,
		ExtraAlarm6:        lp.ExtraAlarm6,
		ExtraAlarm7:        lp.ExtraAlarm7,
		ExtraAlarm8:        lp.ExtraAlarm8,
		SoilLeafAlarm1:     lp.SoilLeafAlarm1,
		SoilLeafAlarm2:     lp.SoilLeafAlarm2,
		SoilLeafAlarm3:     lp.SoilLeafAlarm3,
		SoilLeafAlarm4:     lp.SoilLeafAlarm4,
		TxBatteryStatus:    lp.TxBatteryStatus,
		ConsBatteryVoltage: convConsBatteryVoltage(lp.ConsBatteryVoltage),
		ForecastIcon:       lp.ForecastIcon,
		ForecastRule:       lp.ForecastRule,
		Sunrise:            convSunTime(lp.Sunrise),
		Sunset:             convSunTime(lp.Sunset),
	}

	wc, useWc := calcWindChill(r.OutTemp, r.WindSpeed)
	if useWc {
		r.Windchill = wc
	}

	hi, useHi := calcHeatIndex(r.OutTemp, r.OutHumidity)
	if useHi {
		r.HeatIndex = hi
	}

	return r
}

// ToMap converts a Reading object into a map for later storage
func (r *Reading) ToMap() map[string]interface{} {
	m := make(map[string]interface{})

	v := reflect.ValueOf(*r)

	for i := 0; i < v.NumField(); i++ {
		switch v.Field(i).Kind() {
		case reflect.Float32:
			m[v.Type().Field(i).Name] = v.Field(i).Float()
		case reflect.Uint8:
			m[v.Type().Field(i).Name] = v.Field(i).Uint()
		}
	}

	return m
}

// Used to convert LoopPacket.StormStart to a time.Time.  This conversion
// differes slightly from the conversion used in archive packets.
func convLoopDate(v uint16) time.Time {
	y := int((0x007f & v) + 2000)
	m := int((0xf000 & v) >> 12)
	d := int((0x0f80 & v) >> 7)
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local)
}

func convVal100(v uint16) float32 {
	return float32(v) / 100
}

func convVal1000(v uint16) float32 {
	return float32(v) / 1000
}

func convVal1000Zero(v uint16) float32 {
	if v == 0 {
		return 0
	}
	return float32(v) / 1000
}

func convBigVal(v uint16) float32 {
	if v == 0x7fff {
		return 0
	}
	return float32(v)
}

func convBigVal10(v int16) float32 {
	if v == 0x7fff {
		return 0
	}
	return float32(v) / 10

}

func convBigVal100(v uint16) float32 {
	if v == 0x7fff {
		return 0
	}
	return float32(v) / 100
}

func convLittleVal(v uint8) float32 {
	if v == 0x00ff {
		return 0
	}
	return float32(v)
}

func convLittleVal10(v uint8) float32 {
	if v == 0x00ff {
		return 0
	}
	return float32(v) / 10
}

func convLittleTemp(v uint8) float32 {
	if v == 0x00ff {
		return 0
	}
	return float32(v - 90)
}

func convConsBatteryVoltage(v uint16) float32 {
	return float32((v*300)>>9) / 100.0
}

// Convert today's sunrise or sunset time into a time.Time
func convSunTime(v uint16) time.Time {
	now := time.Now()
	h := int(v / 100)
	m := int(v % 100)
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
}

func calcWindChill(temp float32, windspeed float32) (float32, bool) {
	if (temp > 50) || (windspeed < 3) {
		return 0, false
	}

	w64 := float64(windspeed)
	return (35.74 + (0.6215 * temp) - (35.75 * float32(math.Pow(w64, 0.16))) + (0.4275 * temp * float32(math.Pow(w64, 0.16)))), true
}

func calcHeatIndex(temp float32, humidity float32) (float32, bool) {

	// Heat indices don't make much sense at temps below 77° F
	if temp < 77 {
		return 0.0, false
	}

	// First, we try Steadman's method, which is valid for all heat indices
	// below 80° F
	hi := 0.5 * (temp + 61.0 + ((temp - 68.0) * 1.2) + (humidity + 0.094))
	if hi < 80 {
		// Only return heat index if it's greater than the temperature
		if hi > temp {
			return hi, true
		}
		return 0.0, false
	}

	// Our heat index is > 80, so we need to use the Rothfusz method instead
	c1 := -42.379
	c2 := 2.04901523
	c3 := 10.14333127
	c4 := 0.22475541
	c5 := 0.00683783
	c6 := 0.05481717
	c7 := 0.00122874
	c8 := 0.00085282
	c9 := 0.00000199

	t64 := float64(temp)
	h64 := float64(humidity)

	hi64 := c1 + (c2 * t64) + (c3 * h64) - (c4 * t64 * h64) - (c5 * math.Pow(t64, 2)) - (c6 * math.Pow(h64, 2)) + (c7 * math.Pow(t64, 2) * h64) + (c8 * t64 * math.Pow(h64, 2)) - (c9 * math.Pow(t64, 2) * math.Pow(h64, 2))

	// If RH < 13% and temperature is between 80 and 112, we need to subtract an adjustment
	if humidity < 13 && temp >= 80 && temp <= 112 {
		adj := ((13 - h64) / 4) * math.Sqrt((17-math.Abs(t64-95.0))/17)
		hi64 = hi64 - adj
	} else if humidity > 80 && temp >= 80 && temp <= 87 {
		// Likewise, if RH > 80% and temperature is between 80 and 87, we need to add an adjustment
		adj := ((h64 - 85.0) / 10) * ((87.0 - t64) / 5)
		hi64 = hi64 + adj
	}

	// Only return heat index if it's greater than the temperature
	if hi64 > t64 {
		return float32(hi64), true
	}
	return 0.0, false
}
