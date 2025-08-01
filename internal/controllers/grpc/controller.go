// Package grpc provides gRPC controller for serving weather data via gRPC API.
package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/grpcutil"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	weather "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// Time constants for table selection
const (
	Day   = 24 * time.Hour
	Month = Day * 30
)

// Controller represents the gRPC controller
type Controller struct {
	ctx            context.Context
	wg             *sync.WaitGroup
	configProvider config.ConfigProvider
	Server         *grpc.Server
	DB             *gorm.DB
	DBEnabled      bool
	GRPCConfig     *config.GRPCData
	DeviceManager  *grpcutil.DeviceManager

	weather.UnimplementedWeatherV1Server
}

// NewController creates a new gRPC controller instance
func NewController(ctx context.Context, wg *sync.WaitGroup, configProvider config.ConfigProvider, grpcConfig config.GRPCData) (*Controller, error) {
	ctrl := &Controller{
		ctx:            ctx,
		wg:             wg,
		configProvider: configProvider,
		GRPCConfig:     &grpcConfig,
	}

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	// Create device manager
	ctrl.DeviceManager = grpcutil.NewDeviceManager(cfgData.Devices)

	// Create gRPC server with optional TLS
	if grpcConfig.Cert != "" && grpcConfig.Key != "" {
		creds, err := credentials.NewServerTLSFromFile(grpcConfig.Cert, grpcConfig.Key)
		if err != nil {
			return nil, fmt.Errorf("could not create TLS server from keypair: %v", err)
		}
		ctrl.Server = grpc.NewServer(grpc.Creds(creds))
	} else {
		ctrl.Server = grpc.NewServer()
	}

	// If a TimescaleDB database was configured, set up a GORM DB handle
	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.GetConnectionString() != "" {
		ctrl.DB, err = database.CreateConnection(cfgData.Storage.TimescaleDB.GetConnectionString())
		if err != nil {
			return nil, fmt.Errorf("gRPC controller could not connect to database: %v", err)
		}
		ctrl.DBEnabled = true
	}

	// Register the weather service and reflection
	weather.RegisterWeatherV1Server(ctrl.Server, ctrl)
	reflection.Register(ctrl.Server)

	return ctrl, nil
}

// StartController starts the gRPC controller
func (c *Controller) StartController() error {
	log.Info("Starting gRPC controller...")
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		listenAddr := fmt.Sprintf(":%v", c.GRPCConfig.Port)
		l, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Errorf("gRPC controller could not create listener: %v", err)
			return
		}

		log.Infof("gRPC controller listening on %s", listenAddr)
		if err := c.Server.Serve(l); err != nil {
			log.Errorf("gRPC controller serve error: %v", err)
		}
	}()

	return nil
}

// StopController stops the gRPC controller
func (c *Controller) StopController() {
	log.Info("Stopping gRPC controller...")
	if c.Server != nil {
		c.Server.GracefulStop()
	}
}

// GetWeatherSpan handles requests for weather data over a time span
func (c *Controller) GetWeatherSpan(ctx context.Context, request *weather.WeatherSpanRequest) (*weather.WeatherSpan, error) {
	if !c.DBEnabled {
		return nil, fmt.Errorf("database not enabled")
	}

	var dbFetchedReadings []types.BucketReading
	span := request.SpanDuration.AsDuration()
	spanStart := time.Now().Add(-span)

	// Validate station name using shared utility
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := c.DeviceManager.GetSnowBaseDistance(request.StationName)

	// Select appropriate table based on span duration, similar to restserver
	// Now that stationName is mandatory, all queries will filter by station
	switch {
	case span < 1*Day:
		c.DB.Table("weather_1m").
			Select("*, (? - snowdistance) AS snowdepth", baseDistance).
			Where("bucket > ?", spanStart).
			Where("stationname = ?", request.StationName).
			Order("bucket").
			Find(&dbFetchedReadings)

	case span >= 1*Day && span < 7*Day:
		c.DB.Table("weather_5m").
			Select("*, (? - snowdistance) AS snowdepth", baseDistance).
			Where("bucket > ?", spanStart).
			Where("stationname = ?", request.StationName).
			Order("bucket").
			Find(&dbFetchedReadings)

	case span >= 7*Day && span < 2*Month:
		c.DB.Table("weather_1h").
			Select("*, (? - snowdistance) AS snowdepth", baseDistance).
			Where("bucket > ?", spanStart).
			Where("stationname = ?", request.StationName).
			Order("bucket").
			Find(&dbFetchedReadings)

	default:
		c.DB.Table("weather_1h").
			Select("*, (? - snowdistance) AS snowdepth", baseDistance).
			Where("bucket > ?", spanStart).
			Where("stationname = ?", request.StationName).
			Order("bucket").
			Find(&dbFetchedReadings)
	}

	log.Debugf("gRPC GetWeatherSpan returned %d rows for span %v", len(dbFetchedReadings), span)

	// Transform readings to protobuf format using shared utility
	readings := grpcutil.TransformBucketReadings(&dbFetchedReadings)

	spanResponse := &weather.WeatherSpan{
		SpanStart: timestamppb.New(spanStart),
		Reading:   readings,
	}

	return spanResponse, nil
}

// GetLatestReading handles requests for the latest weather reading
func (c *Controller) GetLatestReading(ctx context.Context, request *weather.LatestReadingRequest) (*weather.WeatherReading, error) {
	if !c.DBEnabled {
		return nil, fmt.Errorf("database not enabled")
	}

	var dbFetchedReadings []types.BucketReading

	// Validate station name using shared utility
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := c.DeviceManager.GetSnowBaseDistance(request.StationName)

	// Build query for latest reading
	c.DB.Table("weather_1m").
		Select("*, (? - snowdistance) AS snowdepth", baseDistance).
		Where("stationname = ?", request.StationName).
		Order("bucket desc").
		Limit(1).
		Find(&dbFetchedReadings)

	if len(dbFetchedReadings) == 0 {
		return nil, fmt.Errorf("no weather readings found")
	}

	// Transform readings using shared utility
	readings := grpcutil.TransformBucketReadings(&dbFetchedReadings)
	if len(readings) > 0 {
		return readings[0], nil
	}

	return nil, fmt.Errorf("no weather readings found")
}

// GetLiveWeather is not implemented for this controller since it's database-based
func (c *Controller) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.WeatherV1_GetLiveWeatherServer) error {
	return fmt.Errorf("live weather streaming not supported by database-based gRPC controller")
}
