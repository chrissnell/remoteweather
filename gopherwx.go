package main

import (
	"flag"
	"log"
	"path/filepath"
)

// Service contains our configuration and runtime objects
type Service struct {
	ws *WeatherStation
}

// New creates a new instance of Service with the given configuration file
func New(filename string) *Service {
	s := &Service{}

	// Read our server configuration
	filename, _ = filepath.Abs(filename)
	cfg, err := NewConfig(filename)
	if err != nil {
		log.Fatalln("Error reading config file.  Did you pass the -config flag?  Run with -h for help.\n", err)
	}

	// Initialize the Controller
	s.ws = NewWeatherStation(cfg)

	return s
}

func main() {
	cfgFile := flag.String("config", "config.yaml", "Path to config file (default: ./config.yaml)")
	flag.Parse()

	s := New(*cfgFile)

	s.ws.StartLoopPolling()
}
