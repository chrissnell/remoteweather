package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// WUConfig describes the YAML-provided configuration for the Weather
// Underground storage backend
type WUConfig struct {
	StationID string `yaml:"station-id,omitempty"`
	Password  string `yaml:"password,omitempty"`
	Endpoint  string `yaml:"endpoint,omitempty"`
}

// WUStorage is our object for WU weather metric storage
type WUStorage struct {
	cfg *Config
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to InfluxDB
func (w WUStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	readingChan := make(chan Reading, 10)
	go w.sendReports(ctx, wg, readingChan)
	return readingChan
}

func (w *WUStorage) sendReports(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			err := w.sendReading(r)
			if err != nil {
				log.Println(err)
			}
		case <-ctx.Done():
			log.Println("Cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in InfluxDB
func (w *WUStorage) sendReading(r Reading) error {
	v := url.Values{}

	// Add our authentication parameters to our URL
	v.Set("ID", w.cfg.Storage.WU.StationID)
	v.Set("PASSWORD", w.cfg.Storage.WU.Password)

	now := time.Now().In(time.UTC)
	v.Set("dateutc", now.Format("2006-01-02 15:04:05"))

	// This is a real-time weather update request (approx 2.5s interval)
	v.Set("action", "updateraw")
	v.Set("realtime", "1")
	v.Set("rtfreq", "2.5")

	// Set some values for our weather metrics
	v.Set("winddir", strconv.FormatInt(int64(r.WindDir), 10))
	v.Set("windspeedmph", strconv.FormatInt(int64(r.WindSpeed), 10))
	v.Set("humidity", strconv.FormatInt(int64(r.InHumidity), 10))
	v.Set("tempf", fmt.Sprintf("%.1f", r.OutTemp))
	v.Set("dailyrainin", fmt.Sprintf("%.2f", r.DayRain))
	v.Set("baromin", fmt.Sprintf("%.2f", r.Barometer))
	v.Set("softwaretype", fmt.Sprintf("gopherwx %v", version))

	resp, err := http.Get(w.cfg.Storage.WU.Endpoint + "?" + v.Encode())
	if err != nil {
		return fmt.Errorf("Error sending report to WU: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("Error reading WU response body: %v", err)
	}

	return nil
}

// NewWUStorage sets up a new WU storage backend
func NewWUStorage(c *Config) (WUStorage, error) {
	w := WUStorage{}

	if c.Storage.WU.StationID == "" {
		return w, fmt.Errorf("You must provide a WU station ID in the configuration file")
	}

	if c.Storage.WU.Password == "" {
		return w, fmt.Errorf("You must provide a WU password in the configuration file")
	}

	if c.Storage.WU.Endpoint == "" {
		c.Storage.WU.Endpoint = "https://rtupdate.wunderground.com/weatherstation/updateweatherstation.php"
	}

	w.cfg = c

	return w, nil
}
