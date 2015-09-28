package model

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
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
	Name string `json:"name"`
	C    net.Conn
	RW   *bufio.ReadWriter
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

func (m *Model) Connect(s string) {
	var err error

	if s == "" {
		log.Fatalln("Must supply a hostname to connect to:", s)
	}

	log.Println("Connecting to:", s)

	m.WS.C, err = net.Dial("tcp", "10.50.0.104:22222")
	if err != nil {
		log.Fatalf("Could not connect to %v.  Error: %v", s, err)
	}

	if m.WS.C == nil {
		log.Fatalln("Conn is nil")
	}

	m.WS.C.SetReadDeadline(time.Now().Add(time.Second * 15))

	writer := bufio.NewWriter(m.WS.C)
	reader := bufio.NewReader(m.WS.C)

	m.WS.RW = bufio.NewReadWriter(reader, writer)
}

func (m *Model) WakeStation() {
	var timer *time.Timer
	var alive bool

	envoy := fmt.Sprint(m.c.Device.Hostname, ":", m.c.Device.Port)

	m.Connect(envoy)

	for alive == false {
		fmt.Println("Waking up station.")
		m.WS.RW.Write([]byte("\n\n\n"))
		m.WS.RW.Flush()
		timer = time.NewTimer(time.Millisecond * 500)
		<-timer.C
		line, err := m.WS.RW.ReadBytes('\r')
		if err != nil {
			log.Fatalln("Could not read from station:", err)
		}
		fmt.Println("This is what we got back:", line)
		if line[0] == 0x0a && line[1] == 0x0d {
			fmt.Println("Station has been awaken.")
			alive = true
		} else {
			fmt.Println("Sleeping 500ms and trying again...")
			timer = time.NewTimer(time.Millisecond * 500)
			<-timer.C
		}
	}

}

func (m *Model) sendData(d []byte) error {
	resp := make([]byte, 1)

	// Write the data
	m.WS.RW.Write(d)
	m.WS.RW.Flush()

	_, err := m.WS.RW.Read(resp)
	if err != nil {
		log.Println("Error reading response:", err)
		return err
	}

	log.Println("RESP ---->", resp)

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
		_, err := buf.WriteTo(m.WS.RW)
		if err != nil {
			return err
		}
		m.WS.RW.Flush()

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
		_, err = buf.WriteTo(m.WS.RW)
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
		_, err = buf.WriteTo(m.WS.RW)
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

func (m *Model) GetDavisLoopPackets(n int) ([]*LoopPacketWithTrend, error) {
	// Make a slice of loop packet maps, n elements long.
	var loopPackets []*LoopPacketWithTrend

	// Wake the console
	m.WakeStation()

	// Request n packets
	m.sendData([]byte(fmt.Sprintf("LOOP %v\n", n)))

	tries := 1

	for l := 1; l <= n; l++ {
		if tries > maxTries {
			return nil, fmt.Errorf("Max retries exeeded while getting loop data")
		}

		// Read up to 99 bytes from the console
		buf := make([]byte, 99)
		_, err := m.WS.RW.Read(buf)
		if err != nil {
			tries++
			return nil, fmt.Errorf("Error while reading from console, LOOP %v: %v", l, err)
		}

		if crc16.Crc16(buf) != 0 {
			return nil, fmt.Errorf("LOOP %v CRC error.  Try #%v", l, tries)
			tries++
		}

		unpacked, err := m.unpackLoopPacket(buf)
		if err != nil {
			tries++
			return nil, fmt.Errorf("Error unpacking loop packet: %v.  Try %v", err, tries)
		}

		loopPackets = append(loopPackets, unpacked)
	}
	return loopPackets, nil
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
