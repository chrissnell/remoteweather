package snowgauge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
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

// Station holds our connection along with some mutexes for operation
type Station struct {
	ctx                context.Context
	wg                 *sync.WaitGroup
	conn               *grpc.ClientConn
	stream             snowgauge.SnowGaugeService_StreamReadingClient
	config             types.DeviceConfig
	configProvider     config.ConfigProvider
	deviceName         string
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
}

func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	station := &Station{
		ctx:                ctx,
		wg:                 wg,
		configProvider:     configProvider,
		deviceName:         deviceName,
		ReadingDistributor: distributor,
		logger:             logger,
	}

	// Load configuration to get device config
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		logger.Fatalf("SnowGauge station [%s] failed to load config: %v", deviceName, err)
	}

	// Find our device configuration
	var deviceConfig *config.DeviceData
	for _, device := range cfgData.Devices {
		if device.Name == deviceName {
			deviceConfig = &device
			break
		}
	}

	if deviceConfig == nil {
		logger.Fatalf("SnowGauge station [%s] device not found in configuration", deviceName)
	}

	// Convert to legacy config format for internal use
	station.config = types.DeviceConfig{
		Name:              deviceConfig.Name,
		Type:              deviceConfig.Type,
		Hostname:          deviceConfig.Hostname,
		Port:              deviceConfig.Port,
		SerialDevice:      deviceConfig.SerialDevice,
		Baud:              deviceConfig.Baud,
		WindDirCorrection: deviceConfig.WindDirCorrection,
		BaseSnowDistance:  deviceConfig.BaseSnowDistance,
		Solar: types.SolarConfig{
			Latitude:  deviceConfig.Solar.Latitude,
			Longitude: deviceConfig.Solar.Longitude,
			Altitude:  deviceConfig.Solar.Altitude,
		},
	}

	if station.config.Hostname == "" || station.config.Port == "" {
		logger.Fatalf("SnowGauge station [%s] must define a hostname and port", station.config.Name)
	}

	return station
}

func (s *Station) StationName() string {
	return s.config.Name
}

// StartWeatherStation connects to the snow gauge and launches the data streaming goroutine
func (s *Station) StartWeatherStation() error {
	log.Infof("Starting SnowGauge weather station [%v]...", s.config.Name)

	// Connect to the snow gauge
	s.ConnectToGauge()

	s.wg.Add(1)
	go s.StreamSnowGaugeReadings()

	return nil
}

// ConnectToGauge establishes a gRPC connection to the snow gauge
func (s *Station) ConnectToGauge() {
	baseDelay := time.Second
	attempt := 0

	for {
		var err error

		// Calculate backoff delay
		delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff
		if delay > time.Second*30 {                    // Cap delay at 30 seconds
			delay = time.Second * 30
		}

		s.conn, err = grpc.NewClient(
			fmt.Sprintf("dns:%v:%v", s.config.Hostname, s.config.Port),
			grpc.WithTransportCredentials(insecure.NewCredentials()), // Use a secure connection in production with certificates
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                2 * time.Minute,  // Increase the ping interval to 2 minutes
				Timeout:             20 * time.Second, // Increase the ping timeout to 20 seconds
				PermitWithoutStream: true,             // Allow keepalive pings even without active streams
			}),
		)
		if err == nil {
			log.Infof("Connected to snow gauge [%v]", s.config.Name)
			client := snowgauge.NewSnowGaugeServiceClient(s.conn)

			// Create a StreamRequest
			req := &snowgauge.StreamRequest{}
			s.stream, err = client.StreamReading(s.ctx, req)
			if err != nil {
				log.Errorf("failed to initiate stream to snow gauge %v: %v", s.config.Name, err)
				log.Errorf("attempting reconnection to snow gauge %v in %v seconds", s.config.Name, delay)
				time.Sleep(delay)
				attempt++
				continue
			}
			// We're connected.
			return
		}
		// Connection failed, so we'll retry with exponential backoff

		log.Errorf("Attempt #%v to connect to snow gauge %v failed. Retrying in %v\n", attempt+1, s.config.Name, delay)
		time.Sleep(delay)
		attempt++
	}
}

func (s *Station) StreamSnowGaugeReadings() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			log.Info("cancellation request received. Cancelling StreamSnowGaugeReadings)")
			return
		default:
			reading, err := s.stream.Recv()
			if err != nil {
				if status.Code(err) == codes.Unavailable {
					log.Errorf("snowgauge %v stream closed by server: %v", s.config.Name, err)
					log.Errorln("sleeping for 5 seconds before reconnecting")
					time.Sleep(5 * time.Second)
					s.ConnectToGauge()
					continue
				}
				log.Errorf("error receiving data from snowgauge %v stream: %v; reconnecting", s.config.Name, err)
				log.Errorln("sleeping for 5 seconds before reconnecting")
				time.Sleep(5 * time.Second)
				s.ConnectToGauge()
				continue
			}
			log.Debugf("Received distance: %d mm\n", reading.Distance)

			// The reading should never ever be larger than our base distance. If it is, we discard it.
			if reading.Distance > int32(s.config.BaseSnowDistance) {
				log.Infof("Discarding invalid snow distance reading from snow gauge %v: %v", s.config.Name, reading.Distance)
				continue
			}

			r := types.Reading{
				Timestamp:    time.Now(),
				StationName:  s.config.Name,
				StationType:  "snowgauge",
				SnowDistance: float32(reading.Distance),
			}

			// Send the reading to the distributor
			log.Debugf("SnowGauge [%s] sending reading to distributor: distance=%.1fmm, depth=%.1fmm",
				s.config.Name, r.SnowDistance, float32(s.config.BaseSnowDistance)-r.SnowDistance)
			s.ReadingDistributor <- r
		}
	}
}
