package model

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/chrissnell/gopherwx/config"
	"github.com/chrissnell/gopherwx/util/crc16"
)

const ACK = 0x06
const RESEND = 0x15

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
	m.WS.C.Write(d)
	resp, err := m.WS.R.ReadBytes(ACK)
	if err != nil {
		return err
	}
	log.Println("Resp:", resp)
	return nil
}

func (m *Model) sendDataWithCRC16(d []byte) error {
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
		resp, err := m.WS.R.ReadByte()
		if err != nil {
			return err
		}
		if resp == ACK {
			log.Println("Sent data and received ACK.")
			return nil
		}
	}

	return fmt.Errorf("I/O error writing data with CRC to device.")
}
