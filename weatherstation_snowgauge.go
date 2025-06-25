package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	snowgauge "github.com/chrissnell/remoteweather/protocols/snowgauge"
)

// SnowGaugeWeatherStation holds our connection along with some mutexes for operation
type SnowGaugeWeatherStation struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	conn               *grpc.ClientConn
	stream             snowgauge.SnowGaugeService_StreamReadingClient
	Config             DeviceConfig
	ReadingDistributor chan Reading
	Logger             *zap.SugaredLogger
}

func NewSnowGaugeWeatherStation(ctx context.Context, wg *sync.WaitGroup, c DeviceConfig, distributor chan Reading, logger *zap.SugaredLogger) (*SnowGaugeWeatherStation, error) {
	s := SnowGaugeWeatherStation{
		ctx:                ctx,
		wg:                 wg,
		Config:             c,
		ReadingDistributor: distributor,
		Logger:             logger,
	}

	if c.Hostname == "" || c.Port == "" {
		return &s, fmt.Errorf("must define a hostname and port for the snow gauge")
	}

	return &s, nil
}

func (w *SnowGaugeWeatherStation) StationName() string {
	return w.Config.Name
}

// StartWeatherStation connects to the snow gauge and launches the data streaming goroutine
func (w *SnowGaugeWeatherStation) StartWeatherStation() error {
	log.Infof("Starting Campbell Scientific weather station [%v]...", w.Config.Name)

	// Connect to the snow gauge
	w.ConnectToGauge()

	w.wg.Add(1)
	go w.StreamSnowGaugeReadings()

	return nil

}

// ConnectToStation establishes a gRPC connection the snow gauge
func (w *SnowGaugeWeatherStation) ConnectToGauge() {
	baseDelay := time.Second
	attempt := 0

	for {
		var err error

		// Calculate backoff delay
		delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff
		if delay > time.Second*30 {                    // Cap delay at 30 seconds
			delay = time.Second * 30
		}

		w.conn, err = grpc.NewClient(
			fmt.Sprintf("dns:%v:%v", w.Config.Hostname, w.Config.Port),
			grpc.WithTransportCredentials(insecure.NewCredentials()), // Use a secure connection in production with certificates
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                2 * time.Minute,  // Increase the ping interval to 2 minutes
				Timeout:             20 * time.Second, // Increase the ping timeout to 20 seconds
				PermitWithoutStream: true,             // Allow keepalive pings even without active streams
			}),
		)
		if err == nil {
			log.Infof("Connected to snow gauge [%v]", w.Config.Name)
			client := snowgauge.NewSnowGaugeServiceClient(w.conn)

			// Create a StreamRequest
			req := &snowgauge.StreamRequest{}
			w.stream, err = client.StreamReading(w.ctx, req)
			if err != nil {
				log.Errorf("failed to initiate stream to snow gauge %v: %v", w.Config.Name, err)
				log.Errorf("attempting reconnection to snow gauge %v in %v seconds", w.Config.Name, delay)
				time.Sleep(delay)
				attempt++
				continue
			}
			// We're connected.
			return
		}
		// Connection failed, so we'll retry with exponential backoff

		log.Errorf("Attempt #%v to connect to snow guage %v failed. Retrying in %v\n", attempt+1, w.Config.Name, delay)
		time.Sleep(delay)
		attempt++
	}

}

func (w *SnowGaugeWeatherStation) StreamSnowGaugeReadings() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			log.Info("cancellation request recieved.  Cancelling StreamSnowGaugeReadings)")
			return
		default:
			reading, err := w.stream.Recv()
			if err != nil {
				if status.Code(err) == codes.Unavailable {
					log.Errorf("snowgauge %v stream closed by server: %v", w.Config.Name, err)
					log.Errorln("sleeping for 5 seconds before reconnecting")
					time.Sleep(5 * time.Second)
					w.ConnectToGauge()
					continue
				}
				log.Errorf("error receiving data from snowgauge %v stream: %v; reconnecting", w.Config.Name, err)
				log.Errorln("sleeping for 5 seconds before reconnecting")
				time.Sleep(5 * time.Second)
				w.ConnectToGauge()
				continue
			}
			log.Debugf("Received distance: %d mm\n", reading.Distance)

			// The reading should never ever be larger than our base distance.  If it is, we discard it.
			if reading.Distance > int32(w.Config.BaseSnowDistance) {
				log.Infof("Discarding invalid snow distance reading from snow gauge %v: %v", w.Config.Name, reading.Distance)
				continue
			}

			r := Reading{
				Timestamp:    time.Now(),
				StationName:  w.Config.Name,
				StationType:  "snowgauge",
				SnowDistance: float32(reading.Distance),
			}

			// Send the reading to the distributor
			w.ReadingDistributor <- r
		}
	}

}
