package grpcreceiver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/internal/weatherstations"
	"github.com/chrissnell/remoteweather/pkg/config"
	pb "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Station implements a gRPC receiver weather station
type Station struct {
	pb.UnimplementedWeatherV1Server
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 *sync.WaitGroup
	server             *grpc.Server
	config             config.DeviceData
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
}

// NewStation creates a new gRPC receiver weather station
func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	deviceConfig := weatherstations.LoadDeviceConfig(configProvider, deviceName, logger)

	if deviceConfig.Port == "" {
		logger.Fatalf("gRPC Receiver station [%s] must define a port", deviceConfig.Name)
	}

	// Create a cancellable context for this specific station
	stationCtx, cancel := context.WithCancel(ctx)

	return &Station{
		ctx:                stationCtx,
		cancel:             cancel,
		wg:                 wg,
		config:             *deviceConfig,
		ReadingDistributor: distributor,
		logger:             logger,
	}
}

// StationName returns the name of this weather station
func (s *Station) StationName() string {
	return s.config.Name
}

// StartWeatherStation starts the gRPC server to receive weather readings
func (s *Station) StartWeatherStation() error {
	s.wg.Add(1)
	defer s.wg.Done()

	// Determine listen address
	listenAddr := s.config.Hostname
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}
	listenAddr = fmt.Sprintf("%s:%s", listenAddr, s.config.Port)

	// Create listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	// Configure gRPC server options
	var opts []grpc.ServerOption

	// Check if TLS cert is configured
	if s.config.TLSCertPath != "" && s.config.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCertPath, s.config.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS cert/key: %w", err)
		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		opts = append(opts, grpc.Creds(creds))
	}

	// Create gRPC server
	s.server = grpc.NewServer(opts...)
	pb.RegisterWeatherV1Server(s.server, s)

	s.logger.Infof("gRPC Receiver station [%s] listening on %s", s.config.Name, listenAddr)

	// Start serving
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Errorf("gRPC server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-s.ctx.Done()
	s.logger.Infof("Shutting down gRPC Receiver station [%s]", s.config.Name)
	s.server.GracefulStop()

	return nil
}

// StopWeatherStation stops the gRPC server
func (s *Station) StopWeatherStation() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// SendWeatherReadings implements the streaming RPC for receiving weather readings
func (s *Station) SendWeatherReadings(stream pb.WeatherV1_SendWeatherReadingsServer) error {
	for {
		reading, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.Empty{})
		}
		if err != nil {
			s.logger.Errorf("Error receiving weather reading: %v", err)
			return err
		}

		// Convert protobuf WeatherReading to internal types.Reading
		internalReading := s.convertToInternalReading(reading)

		// Send to reading distributor
		select {
		case s.ReadingDistributor <- internalReading:
			s.logger.Debugf("Received and distributed reading from station %s", reading.StationName)
		case <-s.ctx.Done():
			return s.ctx.Err()
		}
	}
}

// convertToInternalReading converts a protobuf WeatherReading to internal types.Reading
func (s *Station) convertToInternalReading(pbReading *pb.WeatherReading) types.Reading {
	reading := types.Reading{
		StationName:           pbReading.StationName,
		StationType:           pbReading.StationType,
		Barometer:             pbReading.Barometer,
		InTemp:                pbReading.InsideTemperature,
		InHumidity:            pbReading.InsideHumidity,
		OutTemp:               pbReading.OutsideTemperature,
		OutHumidity:           pbReading.OutsideHumidity,
		WindSpeed:             pbReading.WindSpeed,
		WindSpeed10:           pbReading.WindSpeed10,
		WindDir:               pbReading.WindDirection,
		WindChill:             pbReading.WindChill,
		HeatIndex:             pbReading.HeatIndex,
		ExtraTemp1:            pbReading.ExtraTemp1,
		ExtraTemp2:            pbReading.ExtraTemp2,
		ExtraTemp3:            pbReading.ExtraTemp3,
		ExtraTemp4:            pbReading.ExtraTemp4,
		ExtraTemp5:            pbReading.ExtraTemp5,
		ExtraTemp6:            pbReading.ExtraTemp6,
		ExtraTemp7:            pbReading.ExtraTemp7,
		SoilTemp1:             pbReading.SoilTemp1,
		SoilTemp2:             pbReading.SoilTemp2,
		SoilTemp3:             pbReading.SoilTemp3,
		SoilTemp4:             pbReading.SoilTemp4,
		LeafTemp1:             pbReading.LeafTemp1,
		LeafTemp2:             pbReading.LeafTemp2,
		LeafTemp3:             pbReading.LeafTemp3,
		LeafTemp4:             pbReading.LeafTemp4,
		ExtraHumidity1:        pbReading.ExtraHumidity1,
		ExtraHumidity2:        pbReading.ExtraHumidity2,
		ExtraHumidity3:        pbReading.ExtraHumidity3,
		ExtraHumidity4:        pbReading.ExtraHumidity4,
		ExtraHumidity5:        pbReading.ExtraHumidity5,
		ExtraHumidity6:        pbReading.ExtraHumidity6,
		ExtraHumidity7:        pbReading.ExtraHumidity7,
		RainRate:              pbReading.RainRate,
		RainIncremental:       pbReading.RainIncremental,
		SolarWatts:            pbReading.SolarWatts,
		PotentialSolarWatts:   pbReading.PotentialSolarWatts,
		SolarJoules:           pbReading.SolarJoules,
		UV:                    pbReading.Uv,
		Radiation:             pbReading.Radiation,
		StormRain:             pbReading.StormRain,
		DayRain:               pbReading.DayRain,
		MonthRain:             pbReading.MonthRain,
		YearRain:              pbReading.YearRain,
		DayET:                 pbReading.DayET,
		MonthET:               pbReading.MonthET,
		YearET:                pbReading.YearET,
		SoilMoisture1:         pbReading.SoilMoisture1,
		SoilMoisture2:         pbReading.SoilMoisture2,
		SoilMoisture3:         pbReading.SoilMoisture3,
		SoilMoisture4:         pbReading.SoilMoisture4,
		LeafWetness1:          pbReading.LeafWetness1,
		LeafWetness2:          pbReading.LeafWetness2,
		LeafWetness3:          pbReading.LeafWetness3,
		LeafWetness4:          pbReading.LeafWetness4,
		InsideAlarm:           uint8(pbReading.InsideAlarm),
		RainAlarm:             uint8(pbReading.RainAlarm),
		OutsideAlarm1:         uint8(pbReading.OutsideAlarm1),
		OutsideAlarm2:         uint8(pbReading.OutsideAlarm2),
		ExtraAlarm1:           uint8(pbReading.ExtraAlarm1),
		ExtraAlarm2:           uint8(pbReading.ExtraAlarm2),
		ExtraAlarm3:           uint8(pbReading.ExtraAlarm3),
		ExtraAlarm4:           uint8(pbReading.ExtraAlarm4),
		ExtraAlarm5:           uint8(pbReading.ExtraAlarm5),
		ExtraAlarm6:           uint8(pbReading.ExtraAlarm6),
		ExtraAlarm7:           uint8(pbReading.ExtraAlarm7),
		ExtraAlarm8:           uint8(pbReading.ExtraAlarm8),
		SoilLeafAlarm1:        uint8(pbReading.SoilLeafAlarm1),
		SoilLeafAlarm2:        uint8(pbReading.SoilLeafAlarm2),
		SoilLeafAlarm3:        uint8(pbReading.SoilLeafAlarm3),
		SoilLeafAlarm4:        uint8(pbReading.SoilLeafAlarm4),
		TxBatteryStatus:       uint8(pbReading.TxBatteryStatus),
		ConsBatteryVoltage:    pbReading.ConsBatteryVoltage,
		StationBatteryVoltage: pbReading.StationBatteryVoltage,
		ForecastIcon:          uint8(pbReading.ForecastIcon),
		ForecastRule:          uint8(pbReading.ForecastRule),
		SnowDistance:          pbReading.SnowDistance,
		SnowDepth:             pbReading.SnowDepth,
		ExtraFloat1:           pbReading.ExtraFloat1,
		ExtraFloat2:           pbReading.ExtraFloat2,
		ExtraFloat3:           pbReading.ExtraFloat3,
		ExtraFloat4:           pbReading.ExtraFloat4,
		ExtraFloat5:           pbReading.ExtraFloat5,
		ExtraFloat6:           pbReading.ExtraFloat6,
		ExtraFloat7:           pbReading.ExtraFloat7,
		ExtraFloat8:           pbReading.ExtraFloat8,
		ExtraFloat9:           pbReading.ExtraFloat9,
		ExtraFloat10:          pbReading.ExtraFloat10,
		ExtraText1:            pbReading.ExtraText1,
		ExtraText2:            pbReading.ExtraText2,
		ExtraText3:            pbReading.ExtraText3,
		ExtraText4:            pbReading.ExtraText4,
		ExtraText5:            pbReading.ExtraText5,
		ExtraText6:            pbReading.ExtraText6,
		ExtraText7:            pbReading.ExtraText7,
		ExtraText8:            pbReading.ExtraText8,
		ExtraText9:            pbReading.ExtraText9,
		ExtraText10:           pbReading.ExtraText10,
	}

	// Convert timestamps
	if pbReading.ReadingTimestamp != nil {
		reading.Timestamp = pbReading.ReadingTimestamp.AsTime()
	}
	if pbReading.StormStart != nil {
		reading.StormStart = pbReading.StormStart.AsTime()
	}
	if pbReading.Sunrise != nil {
		reading.Sunrise = pbReading.Sunrise.AsTime()
	}
	if pbReading.Sunset != nil {
		reading.Sunset = pbReading.Sunset.AsTime()
	}

	return reading
}
