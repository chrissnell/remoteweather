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
	deviceName         string
	configProvider     config.ConfigProvider
	ReadingDistributor chan types.Reading
	logger             *zap.SugaredLogger
	registry           *RemoteStationRegistry // For remote station management
}

// NewStation creates a new gRPC receiver weather station
func NewStation(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, deviceName string, distributor chan types.Reading, logger *zap.SugaredLogger) weatherstations.WeatherStation {
	// Validate device has required config
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
		deviceName:         deviceName,
		configProvider:     configProvider,
		ReadingDistributor: distributor,
		logger:             logger,
	}
}

// StationName returns the name of this weather station
func (s *Station) StationName() string {
	return s.deviceName
}

// Capabilities returns the measurement capabilities of this station.
// gRPC receiver stations can have dynamic capabilities based on remote stations.
// For now, we default to Weather for backward compatibility.
// TODO: Query registry to determine actual capabilities from remote stations.
func (s *Station) Capabilities() weatherstations.Capabilities {
	return weatherstations.Capabilities(weatherstations.Weather)
}

// SetRegistry sets the remote station registry for this receiver
func (s *Station) SetRegistry(registry *RemoteStationRegistry) {
	s.registry = registry
}

// InitializeRegistry creates and sets up the remote station registry
func (s *Station) InitializeRegistry() error {
	registry, err := NewRemoteStationRegistry(s.configProvider, s.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize remote station registry: %w", err)
	}
	s.registry = registry
	return nil
}

// StartWeatherStation starts the gRPC server to receive weather readings
func (s *Station) StartWeatherStation() error {
	s.wg.Add(1)
	defer s.wg.Done()

	// Get device configuration
	deviceConfig, err := s.configProvider.GetDevice(s.deviceName)
	if err != nil {
		s.logger.Errorf("Failed to get device config: %v", err)
		return err
	}

	// Initialize registry with config provider
	if err := s.InitializeRegistry(); err != nil {
		s.logger.Errorf("Failed to initialize remote station registry: %v", err)
		return err
	}

	// Determine listen address
	listenAddr := deviceConfig.Hostname
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}
	listenAddr = fmt.Sprintf("%s:%s", listenAddr, deviceConfig.Port)

	// Create listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	// Configure gRPC server options
	var opts []grpc.ServerOption

	// Check if TLS cert is configured
	if deviceConfig.TLSCertPath != "" && deviceConfig.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(deviceConfig.TLSCertPath, deviceConfig.TLSKeyPath)
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

	s.logger.Infof("gRPC Receiver station [%s] listening on %s", s.deviceName, listenAddr)

	// Start serving
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Errorf("gRPC server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-s.ctx.Done()
	s.logger.Infof("Shutting down gRPC Receiver station [%s]", s.deviceName)
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

		// Send to reading distributor for local storage
		select {
		case s.ReadingDistributor <- internalReading:
			s.logger.Debugf("Received and distributed reading from station %s", reading.StationName)
		case <-s.ctx.Done():
			return s.ctx.Err()
		}

		// Check if this is from a registered remote station
		if reading.StationId != "" && s.registry != nil {
			remoteStation := s.registry.GetByID(reading.StationId)
			if remoteStation != nil {
				// Update last seen timestamp
				s.registry.UpdateLastSeen(reading.StationId)
			} else {
				s.logger.Debugf("Received reading from unregistered station ID: %s", reading.StationId)
			}
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
		// Additional temperature sensors
		Temp1:                 pbReading.Temp1,
		Temp2:                 pbReading.Temp2,
		Temp3:                 pbReading.Temp3,
		Temp4:                 pbReading.Temp4,
		Temp5:                 pbReading.Temp5,
		Temp6:                 pbReading.Temp6,
		Temp7:                 pbReading.Temp7,
		Temp8:                 pbReading.Temp8,
		Temp9:                 pbReading.Temp9,
		Temp10:                pbReading.Temp10,
		// Additional soil temperature sensors
		SoilTemp5:             pbReading.SoilTemp5,
		SoilTemp6:             pbReading.SoilTemp6,
		SoilTemp7:             pbReading.SoilTemp7,
		SoilTemp8:             pbReading.SoilTemp8,
		SoilTemp9:             pbReading.SoilTemp9,
		SoilTemp10:            pbReading.SoilTemp10,
		// Additional humidity sensors
		Humidity1:             pbReading.Humidity1,
		Humidity2:             pbReading.Humidity2,
		Humidity3:             pbReading.Humidity3,
		Humidity4:             pbReading.Humidity4,
		Humidity5:             pbReading.Humidity5,
		Humidity6:             pbReading.Humidity6,
		Humidity7:             pbReading.Humidity7,
		Humidity8:             pbReading.Humidity8,
		Humidity9:             pbReading.Humidity9,
		Humidity10:            pbReading.Humidity10,
		// Soil humidity sensors
		SoilHum1:              pbReading.SoilHum1,
		SoilHum2:              pbReading.SoilHum2,
		SoilHum3:              pbReading.SoilHum3,
		SoilHum4:              pbReading.SoilHum4,
		SoilHum5:              pbReading.SoilHum5,
		SoilHum6:              pbReading.SoilHum6,
		SoilHum7:              pbReading.SoilHum7,
		SoilHum8:              pbReading.SoilHum8,
		SoilHum9:              pbReading.SoilHum9,
		SoilHum10:             pbReading.SoilHum10,
		// Additional leaf wetness sensors
		LeafWetness5:          pbReading.LeafWetness5,
		LeafWetness6:          pbReading.LeafWetness6,
		LeafWetness7:          pbReading.LeafWetness7,
		LeafWetness8:          pbReading.LeafWetness8,
		// Soil tension sensors
		SoilTens1:             pbReading.SoilTens1,
		SoilTens2:             pbReading.SoilTens2,
		SoilTens3:             pbReading.SoilTens3,
		SoilTens4:             pbReading.SoilTens4,
		// Agricultural measurements
		GDD:                   int32(pbReading.Gdd),
		ETOS:                  pbReading.Etos,
		ETRS:                  pbReading.Etrs,
		// Leak detection sensors
		Leak1:                 uint8(pbReading.Leak1),
		Leak2:                 uint8(pbReading.Leak2),
		Leak3:                 uint8(pbReading.Leak3),
		Leak4:                 uint8(pbReading.Leak4),
		// Additional battery status
		BattOut:               uint8(pbReading.BattOut),
		BattIn:                uint8(pbReading.BattIn),
		Batt1:                 uint8(pbReading.Batt1),
		Batt2:                 uint8(pbReading.Batt2),
		Batt3:                 uint8(pbReading.Batt3),
		Batt4:                 uint8(pbReading.Batt4),
		Batt5:                 uint8(pbReading.Batt5),
		Batt6:                 uint8(pbReading.Batt6),
		Batt7:                 uint8(pbReading.Batt7),
		Batt8:                 uint8(pbReading.Batt8),
		Batt9:                 uint8(pbReading.Batt9),
		Batt10:                uint8(pbReading.Batt10),
		Batt25:                uint8(pbReading.Batt25),
		BattLightning:         uint8(pbReading.BattLightning),
		BatLeak1:              uint8(pbReading.BatLeak1),
		BatLeak2:              uint8(pbReading.BatLeak2),
		BatLeak3:              uint8(pbReading.BatLeak3),
		BatLeak4:              uint8(pbReading.BatLeak4),
		BattSM1:               uint8(pbReading.BattSM1),
		BattSM2:               uint8(pbReading.BattSM2),
		BattSM3:               uint8(pbReading.BattSM3),
		BattSM4:               uint8(pbReading.BattSM4),
		BattCO2:               uint8(pbReading.BattCO2),
		BattCellGateway:       uint8(pbReading.BattCellGateway),
		// Pressure measurements
		BaromRelIn:            pbReading.BaromRelIn,
		BaromAbsIn:            pbReading.BaromAbsIn,
		// Relay states
		Relay1:                uint8(pbReading.Relay1),
		Relay2:                uint8(pbReading.Relay2),
		Relay3:                uint8(pbReading.Relay3),
		Relay4:                uint8(pbReading.Relay4),
		Relay5:                uint8(pbReading.Relay5),
		Relay6:                uint8(pbReading.Relay6),
		Relay7:                uint8(pbReading.Relay7),
		Relay8:                uint8(pbReading.Relay8),
		Relay9:                uint8(pbReading.Relay9),
		Relay10:               uint8(pbReading.Relay10),
		// Air quality measurements
		PM25:                  pbReading.Pm25,
		PM25_24H:              pbReading.Pm25_24H,
		PM25In:                pbReading.Pm25In,
		PM25In24H:             pbReading.Pm25In24H,
		PM25InAQIN:            pbReading.Pm25InAQIN,
		PM25In24HAQIN:         pbReading.Pm25In24HAQIN,
		PM10InAQIN:            pbReading.Pm10InAQIN,
		PM10In24HAQIN:         pbReading.Pm10In24HAQIN,
		CO2:                   pbReading.Co2,
		CO2InAQIN:             int32(pbReading.Co2InAQIN),
		CO2In24HAQIN:          int32(pbReading.Co2In24HAQIN),
		PMInTempAQIN:          pbReading.PmInTempAQIN,
		PMInHumidityAQIN:      int32(pbReading.PmInHumidityAQIN),
		AQIPM25AQIN:           int32(pbReading.AqiPM25AQIN),
		AQIPM2524HAQIN:        int32(pbReading.AqiPM2524HAQIN),
		AQIPM10AQIN:           int32(pbReading.AqiPM10AQIN),
		AQIPM1024HAQIN:        int32(pbReading.AqiPM1024HAQIN),
		AQIPM25In:             int32(pbReading.AqiPM25In),
		AQIPM25In24H:          int32(pbReading.AqiPM25In24H),
		// Lightning data
		LightningDay:          int32(pbReading.LightningDay),
		LightningHour:         int32(pbReading.LightningHour),
		LightningDistance:     pbReading.LightningDistance,
		// Time zone and timestamp
		TZ:                    pbReading.Tz,
		DateUTC:               pbReading.DateUTC,
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
	if pbReading.LightningTime != nil {
		reading.LightningTime = pbReading.LightningTime.AsTime()
	}

	return reading
}

// RegisterRemoteStation handles remote station registration
func (s *Station) RegisterRemoteStation(ctx context.Context, config *pb.RemoteStationConfig) (*pb.RegistrationAck, error) {
	if s.registry == nil {
		return &pb.RegistrationAck{
			Success: false,
			Message: "Remote station registry not initialized",
		}, nil
	}

	// Register the station
	stationID, err := s.registry.Register(config)
	if err != nil {
		s.logger.Errorf("Failed to register remote station %s: %v", config.StationName, err)
		return &pb.RegistrationAck{
			Success: false,
			Message: fmt.Sprintf("Registration failed: %v", err),
		}, nil
	}

	s.logger.Infof("Successfully registered remote station %s with ID %s", config.StationName, stationID)
	return &pb.RegistrationAck{
		Success:   true,
		StationId: stationID,
		Message:   "Station registered successfully",
	}, nil
}
