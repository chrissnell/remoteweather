package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// Service contains our configuration and runtime objects
type Service struct {
	ws  *WeatherStation
	sto *Storage
}

// NewService creates a new instance of Service with the given configuration file
func NewService(cfg *Config, sto *Storage) *Service {
	s := &Service{}

	// Initialize the Controller
	s.ws = NewWeatherStation(*cfg, sto)

	return s
}

func main() {
	var wg sync.WaitGroup

	cfgFile := flag.String("config", "config.yaml", "Path to config file (default: ./config.yaml)")
	flag.Parse()

	// Read our server configuration
	filename, _ := filepath.Abs(*cfgFile)
	cfg, err := NewConfig(filename)
	if err != nil {
		log.Fatalln("Error reading config file.  Did you pass the -config flag?  Run with -h for help.\n", err)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	sto, err := NewStorage(ctx, &wg, &cfg)
	if err != nil {
		log.Fatalln(err)
	}

	s := NewService(&cfg, sto)

	go s.ws.StartLoopPolling()

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func(cancel context.CancelFunc) {
		<-sigs
		cancel()
		close(done)
	}(cancel)

	<-done
	wg.Wait()

}
