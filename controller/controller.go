package controller

import (
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
