package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/chrissnell/gopherwx/config"
	"github.com/chrissnell/gopherwx/controller"
	"github.com/chrissnell/gopherwx/model"
)

// Service contains our configuration and runtime objects
type Service struct {
	Config     config.Config
	controller *controller.Controller
	model      *model.Model
}

// New creates a new instance of Service with the given configuration file
func New(filename string) *Service {
	s := &Service{}

	// Read our server configuration
	filename, _ = filepath.Abs(filename)
	cfg, err := config.New(filename)
	if err != nil {
		log.Fatalln("Error reading config file.  Did you pass the -config flag?  Run with -h for help.\n", err)
	}
	s.Config = cfg

	s.model = model.New(s.Config)

	// Initialize the Controller
	s.controller = controller.New(s.Config, s.model)

	return s
}

func main() {
	cfgFile := flag.String("config", "config.yaml", "Path to config file (default: ./config.yaml)")
	flag.Parse()

	s := New(*cfgFile)

	s.model.WakeStation()
}
