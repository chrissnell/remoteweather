package snowgauge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	snowgauge "github.com/chrissnell/remoteweather/protocols/snowgauge"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type Station struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	conn               *grpc.ClientConn
	stream             snowgauge.SnowGaugeService_StreamReadingClient
	config             config.DeviceData
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
}

func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	if deviceConfig.Hostname == "" || deviceConfig.Port == "" {
		logger.Fatalf("SnowGauge station [%s] must define a hostname and port", deviceConfig.Name)
	}

	return &Station{
		ctx:                ctx,
		wg:                 wg,
		config:             *deviceConfig,
		ReadingDistributor: distributor,
		logger:             logger,
	}
}

func (s *Station) StationName() string {
	return s.config.Name
}

func (s *Station) StartWeatherStation() error {
	s.logger.Infof("Starting SnowGauge weather station [%s]", s.config.Name)

	s.ConnectToGauge()

	s.wg.Add(1)
	go s.StreamSnowGaugeReadings()

	return nil
}

func (s *Station) ConnectToGauge() {
	baseDelay := time.Second
	attempt := 0

	for {
		var err error

		delay := baseDelay * time.Duration(1<<attempt)
		if delay > time.Second*30 {
			delay = time.Second * 30
		}

		s.conn, err = grpc.NewClient(
			fmt.Sprintf("dns:%v:%v", s.config.Hostname, s.config.Port),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                2 * time.Minute,
				Timeout:             20 * time.Second,
				PermitWithoutStream: true,
			}),
		)
		if err == nil {
			s.logger.Infof("Connected to snow gauge [%s]", s.config.Name)
			client := snowgauge.NewSnowGaugeServiceClient(s.conn)

			req := &snowgauge.StreamRequest{}
			s.stream, err = client.StreamReading(s.ctx, req)
			if err != nil {
				s.logger.Errorf("failed to initiate stream to snow gauge %s: %v", s.config.Name, err)
				s.logger.Errorf("attempting reconnection to snow gauge %s in %v seconds", s.config.Name, delay)

				select {
				case <-s.ctx.Done():
					s.logger.Info("cancellation request received during stream retry")
					return
				case <-time.After(delay):
				}
				attempt++
				continue
			}
			return
		}

		s.logger.Errorf("Attempt #%v to connect to snow gauge %s failed. Retrying in %v", attempt+1, s.config.Name, delay)

		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received during connection retry")
			return
		case <-time.After(delay):
		}
		attempt++
	}
}

func (s *Station) StreamSnowGaugeReadings() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("cancellation request received")
			return
		default:
			reading, err := s.stream.Recv()
			if err != nil {
				if status.Code(err) == codes.Unavailable {
					s.logger.Errorf("snowgauge %s stream closed by server: %v", s.config.Name, err)
					s.logger.Errorln("sleeping for 5 seconds before reconnecting")

					select {
					case <-s.ctx.Done():
						s.logger.Info("cancellation request received during reconnection wait")
						return
					case <-time.After(5 * time.Second):
					}
					s.ConnectToGauge()
					continue
				}
				s.logger.Errorf("error receiving data from snowgauge %s stream: %v; reconnecting", s.config.Name, err)
				s.logger.Errorln("sleeping for 5 seconds before reconnecting")

				select {
				case <-s.ctx.Done():
					s.logger.Info("cancellation request received during reconnection wait")
					return
				case <-time.After(5 * time.Second):
				}
				s.ConnectToGauge()
				continue
			}
			s.logger.Debugf("Received distance: %d mm", reading.Distance)

			if reading.Distance > int32(s.config.BaseSnowDistance) {
				s.logger.Infof("Discarding invalid snow distance reading from snow gauge %s: %v", s.config.Name, reading.Distance)
				continue
			}

			r := types.Reading{
				Timestamp:    time.Now(),
				StationName:  s.config.Name,
				StationType:  "snowgauge",
				SnowDistance: float32(reading.Distance),
			}

			s.logger.Debugf("SnowGauge [%s] sending reading: distance=%.0fmm, depth=%.0fmm",
				s.config.Name, r.SnowDistance, float32(s.config.BaseSnowDistance)-r.SnowDistance)
			s.ReadingDistributor <- r
		}
	}
}
