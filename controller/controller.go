package controller

import (
	"log"

	"github.com/chrissnell/gopherwx/config"
	"github.com/chrissnell/gopherwx/model"
)

type Controller struct {
	c config.Config
	m *model.Model
}

// New will create a new Controller
func New(config config.Config, m *model.Model) *Controller {

	c := &Controller{
		c: config,
		m: m,
	}

	return c
}

func (c *Controller) StartLoopPolling() {
	packetChan := make(chan *model.Reading)

	// Wake the console
	c.m.WakeStation()

	go c.GetLoopPackets(packetChan)
	c.ProcessLoopPackets(packetChan)
}

func (c *Controller) ProcessLoopPackets(packetChan <-chan *model.Reading) {

	for {
		select {
		case p := <-packetChan:
			log.Printf("Packet: %+v", p)
		}
	}

}

func (c *Controller) GetLoopPackets(packetChan chan<- *model.Reading) {
	for {
		c.m.GetDavisLoopPackets(20, packetChan)
	}
}
