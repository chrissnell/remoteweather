package model

// Ported from Tom Keffer's weewx
// https://github.com/weewx/weewx/blob/master/bin/weewx/drivers/vantage.py

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/gopherwx/config"
	"github.com/chrissnell/gopherwx/util/crc16"
)

const (
	ACK    = "\x06"
	RESEND = "\x15"

	maxTries = 3
)

type WeatherStation struct {
	Name         string `json:"name"`
	C            net.Conn
	RW           *bufio.ReadWriter
	connecting   bool
	connectingMu sync.RWMutex
	connected    bool
	connectedMu  sync.RWMutex
}

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

// Alternative loop packet type with 3-hour barometer trend
type LoopPacketWithTrend struct {
	LoopPacket
	Trend int8
}

// Reading is a LoopPacketWithTrend that has been converted to human-readable values
// Administrative elements (e.g. LoopType) not related to weather readings have been
// left out.
type Reading struct {
	Barometer          float32
	InTemp             float32
	InHumidity         float32
	OutTemp            float32
	WindSpeed          float32
	WindSpeed10        float32
	WindDir            float32
	ExtraTemp1         float32
	ExtraTemp2         float32
	ExtraTemp3         float32
	ExtraTemp4         float32
	ExtraTemp5         float32
	ExtraTemp6         float32
	ExtraTemp7         float32
	SoilTemp1          float32
	SoilTemp2          float32
	SoilTemp3          float32
	SoilTemp4          float32
	LeafTemp1          float32
	LeafTemp2          float32
	LeafTemp3          float32
	LeafTemp4          float32
	OutHumidity        float32
	ExtraHumidity1     float32
	ExtraHumidity2     float32
	ExtraHumidity3     float32
	ExtraHumidity4     float32
	ExtraHumidity5     float32
	ExtraHumidity6     float32
	ExtraHumidity7     float32
	RainRate           float32
	UV                 float32
	Radiation          float32
	StormRain          float32
	StormStart         time.Time
	DayRain            float32
	MonthRain          float32
	YearRain           float32
	DayET              float32
	MonthET            float32
	YearET             float32
	SoilMoisture1      float32
	SoilMoisture2      float32
	SoilMoisture3      float32
	SoilMoisture4      float32
	LeafWetness1       float32
	LeafWetness2       float32
	LeafWetness3       float32
	LeafWetness4       float32
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
	ConsBatteryVoltage float32
	ForecastIcon       uint8
	ForecastRule       uint8
	Sunrise            time.Time
	Sunset             time.Time
}

// Model contains the data model with the associated etcd Client
type Model struct {
	c  config.Config
	WS *WeatherStation
}

// New creates a new data model with a new DB connection and Kube API client
func New(c config.Config) *Model {

	ws := new(WeatherStation)

	m := &Model{
		WS: ws,
		c:  c,
	}

	return m
}

func (l *LoopPacketWithTrend) String() string {
	return fmt.Sprint("Outside Temp ", convBigVal10(l.OutTemp))
}

func (m *Model) Connect() {
	var err error

	console := fmt.Sprint(m.c.Device.Hostname, ":", m.c.Device.Port)

	m.WS.connectingMu.RLock()

	if m.WS.connecting {
		m.WS.connectingMu.RUnlock()
		log.Println("Skipping reconnect since a connection attempt is already in progress")
		return
	} else {
		// A connection attempt is not in progress so we'll start a new one
		m.WS.connectingMu.RUnlock()
		m.WS.connectingMu.Lock()
		m.WS.connecting = true
		m.WS.connectingMu.Unlock()

		log.Println("Connecting to:", console)

		for {
			m.WS.C, err = net.DialTimeout("tcp", console, 60*time.Second)
			if err != nil {
				log.Printf("Could not connect to %v.  Error: %v", console, err)
				log.Println("Sleeping 5 seconds and trying again.")
				time.Sleep(5 * time.Second)
			} else {
				// We're connected now so we set connected to true and connecting to false
				m.WS.connectedMu.Lock()
				defer m.WS.connectedMu.Unlock()
				m.WS.connected = true
				m.WS.connectingMu.Lock()
				defer m.WS.connectingMu.Unlock()
				m.WS.connecting = false

				// Create a ReadWriter for our connection and set a ReadDeadline
				writer := bufio.NewWriter(m.WS.C)
				reader := bufio.NewReader(m.WS.C)
				m.WS.RW = bufio.NewReadWriter(reader, writer)
				m.WS.C.SetReadDeadline(time.Now().Add(time.Second * 30))
				return
			}
		}
	}
}

func (m *Model) Write(p []byte) (nn int, err error) {
	for {
		nn, err = m.WS.RW.Write(p)
		m.WS.RW.Flush()
		if err != nil {
			// We must not be connected
			log.Println("Error writing to console:", err)
			log.Println("Attempting to reconnect...")
			m.Connect()
		} else {
			// Write was successful
			return nn, err
		}
	}
}

func (m *Model) WakeStation() {
	var alive bool

	m.Connect()

	resp := make([]byte, 1024)

	for alive == false {
		// Flush buffers
		m.WS.RW.Flush()

		fmt.Println("Waking up station.")
		m.WS.RW.Write([]byte("\n"))
		m.WS.RW.Flush()
		_, err := m.WS.C.Read(resp)
		if err != nil {
			log.Fatalln("Could not read from station:", err)
		}
		fmt.Println("This is what we got back:", resp)
		if resp[0] == 0x0a && resp[1] == 0x0d {
			fmt.Println("Station has been awaken.")
			alive = true
			return
		} else {
			fmt.Println("Sleeping 500ms and trying again...")
			time.Sleep(500 * time.Millisecond)
		}
	}

}

func (m *Model) sendData(d []byte) error {
	resp := make([]byte, 1)

	// Write the data
	m.Write(d)
	m.WS.RW.Flush()

	_, err := m.WS.RW.Read(resp)
	if err != nil {
		log.Println("Error reading response:", err)
		return err
	}

	fmt.Println("sendData RESP:", resp)

	// See if it was ACKed
	if resp[0] != 0x06 {
		log.Println("No <ACK> received from console")
	}
	return nil
}

func (m *Model) sendDataWithCRC16(d []byte) error {
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
		_, err := buf.WriteTo(m)
		if err != nil {
			return err
		}

		_, err = m.WS.RW.Read(resp)
		if err != nil {
			log.Println("Error reading response:", err)
			return err
		}

		if resp[0] != ACK[0] {
			log.Println("No <ACK> was received from console")
			return nil
		} else {
			log.Println("Send data to console and recieved ACK.")
		}
	}

	return fmt.Errorf("I/O error writing data with CRC to device.")
}

func (m *Model) sendCommand(command []byte) (error, []string) {
	var err error
	var resp []byte

	// We'll write to a Buffer and then dump the buffer to the device
	buf := new(bytes.Buffer)

	// We'll try to send it up to maxTries times before erroring out
	for i := 0; i <= maxTries; i++ {
		m.WakeStation()

		// First, write the data
		_, err = buf.Write(command)

		// Write the buffer to the device
		_, err = buf.WriteTo(m)
		if err != nil {
			return err, nil
		}
		m.WS.RW.Flush()

		// Sleep for 500ms to wait for the device to react and fill its buffer
		time.Sleep(500 * time.Millisecond)

		_, err = m.WS.RW.Read(resp)
		if err != nil {
			return err, nil
		}

		parts := strings.Split(string(resp), "\n\r")

		if parts[0] == "OK" {
			return nil, parts[1:]
		} else {
			return err, nil
		}
	}
	log.Println("Tried three times to send command but failed.")
	return err, nil
}

func (m *Model) getDataWithCRC16(numBytes int64, prompt string) ([]byte, error) {
	var err error

	buf := new(bytes.Buffer)

	if prompt != "" {
		// We'll write to a Buffer and then dump it to the device
		_, err = buf.WriteString(prompt)
		if err != nil {
			return nil, err
		}

		// Write the buffer to the device
		_, err = buf.WriteTo(m)
		if err != nil {
			return nil, err
		}
		m.WS.RW.Flush()

	}

	// We're going to try reading data from the device maxTries times...
	for i := 1; i <= maxTries; i++ {

		// If it's not our first attempt at reading from the console, we send a RESEND command
		// to goad the console into responding.
		if i > 1 {
			_, err = buf.Write([]byte(RESEND))
			if err != nil {
				log.Println("Could not write RESEND command to buffer")
				return nil, err
			}
			// Write the buffer to the console
			_, err = buf.WriteTo(m.WS.RW)
			if err != nil {
				log.Println("Could not write buffer to console")
				return nil, err
			}

			checkBytes := make([]byte, numBytes)

			_, err := m.WS.RW.Read(checkBytes)
			if err != nil {
				return nil, err
			}

			// Do a CRC16 check on data we read and return data if it passes
			if crc16.Crc16(checkBytes) == uint16(0) {
				return checkBytes, nil
			}

			// We didn't pass the CRC check so we loop again.
			log.Println("The data read did not pass the CRC16 check")
		}
	}

	// We failed at reading data from the console
	return nil, fmt.Errorf("Failed to read any data from the console after %v attempts.", maxTries)
}

func (m *Model) GetDavisLoopPackets(n int, packetChan chan<- *Reading) error {
	// Make a slice of loop packet maps, n elements long.
	//var loopPackets []*LoopPacketWithTrend

	log.Println("Initiating LOOP -->", n)
	// Request n packets
	m.sendData([]byte(fmt.Sprintf("LOOP %v\n", n)))
	m.WS.RW.Flush()

	time.Sleep(1 * time.Second)

	tries := 1

	for l := 0; l < n; l++ {

		time.Sleep(1 * time.Second)

		if tries > maxTries {
			log.Println("Max retries exeeded while getting loop data")
			return nil
		}

		err := m.WS.C.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			log.Println("Error setting read deadline:", err)
		}

		// Read 99 bytes from the console
		buf := make([]byte, 99)
		_, err = m.WS.RW.Read(buf)
		//_, err = io.ReadAtLeast(m.WS.C, buf, 99)
		if err != nil {
			tries++
			log.Printf("Error while reading from console, LOOP %v: %v", l, err)
			return nil
		}

		if buf[95] != 0x0A && buf[96] != 0x0D {
			log.Println("End-of-packet signature not found; rejecting.")
		} else {

			if crc16.Crc16(buf) != 0 {
				log.Printf("LOOP %v CRC error.  Try #%v", l, tries)
				tries++
				continue
			}

			unpacked, err := m.unpackLoopPacket(buf)
			if err != nil {
				tries++
				log.Printf("Error unpacking loop packet: %v.  Try %v", err, tries)
				continue
			}

			tries = 1

			r := convValues(unpacked)

			packetChan <- r
			//loopPackets = append(loopPackets, unpacked)
		}
	}
	return nil
}

func (m *Model) unpackLoopPacket(p []byte) (*LoopPacketWithTrend, error) {
	var trend int8
	var isFlavorA bool

	lp := new(LoopPacket)
	lpwt := new(LoopPacketWithTrend)

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
	if bytes.Compare(peek, []byte{80}) == 0 {
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

func convValues(lp *LoopPacketWithTrend) *Reading {
	r := &Reading{
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

	return r
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
	} else {
		return float32(v) / 1000
	}
}

func convBigVal(v uint16) float32 {
	if v == 0x7fff {
		return 0
	} else {
		return float32(v)
	}
}

func convBigVal10(v int16) float32 {
	if v == 0x7fff {
		return 0
	} else {
		return float32(v) / 10
	}
}

func convBigVal100(v uint16) float32 {
	if v == 0x7fff {
		return 0
	} else {
		return float32(v) / 100
	}
}

func convLittleVal(v uint8) float32 {
	if v == 0x00ff {
		return 0
	} else {
		return float32(v)
	}
}

func convLittleVal10(v uint8) float32 {
	if v == 0x00ff {
		return 0
	} else {
		return float32(v) / 10
	}
}

func convLittleTemp(v uint8) float32 {
	if v == 0x00ff {
		return 0
	} else {
		return float32(v - 90)
	}
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
