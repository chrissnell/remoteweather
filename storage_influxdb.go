package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/influxdata/influxdb/client/v2"
)

// InfluxDBConfig describes the YAML-provided configuration for a InfluxDB
// storage backend
type InfluxDBConfig struct {
	Scheme   string `yaml:"scheme"`
	Host     string `yaml:"host"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Database string `yaml:"database"`
	Port     int    `yaml:"port,omitempty"`
	Protocol string `yaml:"protocol,omitempty"`
}

// InfluxDBStorage holds the configuration for a InfluxDB storage backend
type InfluxDBStorage struct {
	InfluxDBConn client.Client
	DBName       string
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to InfluxDB
func (i InfluxDBStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting InfluxDB storage engine...")
	readingChan := make(chan Reading, 10)
	go i.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (i InfluxDBStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			err := i.StoreReading(r)
			if err != nil {
				log.Error(err)
			}
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// StoreReading stores a reading value in InfluxDB
func (i InfluxDBStorage) StoreReading(r Reading) error {

	fields := r.ToMap()

	// Set the tags for this data point
	tags := map[string]string{"station": r.StationName}

	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  i.DBName,
		Precision: "s",
	})
	if err != nil {
		return fmt.Errorf("could not create a point batch for InfluxDB: %v", err)
	}

	pt, err := client.NewPoint("wx_reading", tags, fields, r.Timestamp)

	if err != nil {
		return fmt.Errorf("could not create data point for InfluxDB: %v", err)
	}

	bp.AddPoint(pt)

	// Write the batch
	err = i.InfluxDBConn.Write(bp)
	if err != nil {
		return fmt.Errorf("could not write data point to InfluxDB: %v", err)

	}

	return nil
}

// NewInfluxDBStorage sets up a new InfluxDB storage backend
func NewInfluxDBStorage(c *Config) (InfluxDBStorage, error) {
	var err error
	i := InfluxDBStorage{}

	i.DBName = c.Storage.InfluxDB.Database

	switch c.Storage.InfluxDB.Protocol {
	case "http":
		url := fmt.Sprintf("%v://%v:%v", c.Storage.InfluxDB.Scheme, c.Storage.InfluxDB.Host, c.Storage.InfluxDB.Port)
		i.InfluxDBConn, err = client.NewHTTPClient(client.HTTPConfig{
			Addr:     url,
			Username: c.Storage.InfluxDB.Username,
			Password: c.Storage.InfluxDB.Password,
		})
		if err != nil {
			log.Warn("warning: could not create InfluxDB connection!", err)
			return InfluxDBStorage{}, err
		}
	case "udp":
		u := client.UDPConfig{
			Addr: fmt.Sprintf("%v:%v", c.Storage.InfluxDB.Host, c.Storage.InfluxDB.Port),
		}
		i.InfluxDBConn, err = client.NewUDPClient(u)
		if err != nil {
			log.Warn("warning: could not create InfluxDB connection.", err)
			return InfluxDBStorage{}, err
		}
	default:
		url := fmt.Sprintf("%v://%v:%v", c.Storage.InfluxDB.Scheme, c.Storage.InfluxDB.Host, c.Storage.InfluxDB.Port)
		i.InfluxDBConn, err = client.NewHTTPClient(client.HTTPConfig{
			Addr:     url,
			Username: c.Storage.InfluxDB.Username,
			Password: c.Storage.InfluxDB.Password,
		})
		if err != nil {
			log.Warn("warning: could not create InfluxDB connection!", err)
			return InfluxDBStorage{}, err
		}
	}

	return i, nil
}
