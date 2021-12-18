package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

const version = "3.0-" + runtime.GOOS + "/" + runtime.GOARCH

var zapLogger *zap.Logger
var log *zap.SugaredLogger

// Service contains our configuration and runtime objects
type Service struct {
	ws *WeatherStation
}

// NewService creates a new instance of Service with the given configuration file
func NewService(cfg *Config, sto *StorageManager, logger *zap.SugaredLogger) *Service {
	s := &Service{}

	// Initialize the Controller
	s.ws = NewWeatherStation(*cfg, sto, logger)

	return s
}

var debug *bool

func main() {
	var wg sync.WaitGroup
	var err error

	cfgFile := flag.String("config", "config.yaml", "Path to config file (default: ./config.yaml)")
	debug = flag.Bool("debug", false, "Turn on debugging output")
	flag.Parse()

	// Set up our logger
	if *debug {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("can't initialize zap logger: %v", err)
		panic(0)
	}
	defer zapLogger.Sync()
	log = zapLogger.Sugar()

	// Read our server configuration
	filename, _ := filepath.Abs(*cfgFile)
	cfg, err := NewConfig(filename)
	if err != nil {
		log.Fatal("error reading config file.  Did you pass the -config flag?  Run with -h for help.\n", err)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	sto, err := NewStorageManager(ctx, &wg, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	s := NewService(&cfg, sto, log)

	go s.ws.StartLoopPolling()

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func(cancel context.CancelFunc) {
		// If we get a SIGINT or SIGTERM, cancel the context and unblock 'done'
		// to trigger a program shutdown
		<-sigs
		cancel()
		close(done)
	}(cancel)

	// Wait for 'done' to unblock before terminating
	<-done

	// Also wait for all of our workers to terminate before terminating the program
	wg.Wait()

}
