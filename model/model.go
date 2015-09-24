package model

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/chrissnell/gopherwx/config"
	"github.com/chrissnell/gopherwx/util/crc16"
)

const ACK = "\x06"
const RESEND = "\x15"

type WeatherStation struct {
	Name string `json:"name"`
	C    net.Conn
	R    *bufio.Reader
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

	m.WS.R = bufio.NewReader(m.WS.C)
}

func (m *Model) WakeStation() {
	var timer *time.Timer
	var alive bool

	envoy := fmt.Sprint(m.c.Device.Hostname, ":", m.c.Device.Port)

	m.Connect(envoy)

	for alive == false {
		fmt.Println("Waking up station.")
		m.WS.C.Write([]byte("\n\n\n"))
		timer = time.NewTimer(time.Millisecond * 500)
		<-timer.C
		line, err := m.WS.R.ReadBytes('\r')
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
	var resp []byte

	// Write the data
	m.WS.C.Write(d)

	_, err := m.WS.R.Read(resp)
	if err != nil {
		log.Println("Error reading response:", err)
		return err
	}

	// See if it was ACKed
	if resp[0] != ACK[0] {
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

	for i := 0; i <= 3; i++ {
		_, err := buf.WriteTo(m.WS.C)
		if err != nil {
			return err
		}

		_, err = m.WS.R.Read(resp)
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

	// We'll try to send it up to three times before erroring out
	for i := 0; i <= 3; i++ {
		m.WakeStation()

		// First, write the data
		_, err = buf.Write(command)

		// Write the buffer to the device
		_, err = buf.WriteTo(m.WS.C)
		if err != nil {
			return err, nil
		}

		// Sleep for 500ms to wait for the device to react and fill its buffer
		time.Sleep(500 * time.Millisecond)

		_, err = m.WS.R.Read(resp)
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
	var checkBytes []byte

	buf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)

	if prompt != "" {
		// We'll write to a Buffer and then dump it to the device
		_, err = buf.WriteString(prompt)
		if err != nil {
			return nil, err
		}

		// Write the buffer to the device
		_, err = buf.WriteTo(m.WS.C)
		if err != nil {
			return nil, err
		}

	}

	// We're going to try reading data from the device three times...
	for i := 1; i <= 3; i++ {

		// If it's not our first attempt at reading from the console, we send a RESEND command
		// to goad the console into responding.
		if i > 1 {
			_, err = buf.Write([]byte(RESEND))
			if err != nil {
				log.Println("Could not write RESEND command to buffer")
				return nil, err
			}
			// Write the buffer to the console
			_, err = buf.WriteTo(m.WS.C)
			if err != nil {
				log.Println("Could not write buffer to console")
				return nil, err
			}

			_, err := io.CopyN(outBuf, m.WS.R, numBytes)
			if err != nil {
				return nil, err
			}

			_, err = outBuf.Write(checkBytes)
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
	return nil, fmt.Errorf("Failed to read any data from the console after 3 attempts.")
}
