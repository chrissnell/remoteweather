// Package grpcstream provides gRPC storage backend for streaming weather data to remote clients.
package grpcstream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/grpcutil"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/storage"
	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/config"
	weather "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Storage implements a gRPC storage backend
type Storage struct {
	ClientChans     []chan types.Reading
	ClientChanMutex sync.RWMutex
	DBClient        *database.Client
	DBEnabled       bool
	Server          *grpc.Server
	GRPCConfig      *config.GRPCData
	DeviceManager   *grpcutil.DeviceManager

	weather.UnimplementedWeatherV1Server
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (g *Storage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting gRPC storage engine...")
	readingChan := make(chan types.Reading)
	go storage.ProcessReadings(ctx, wg, readingChan, g.processReading, "gRPC")
	return readingChan
}

func (g *Storage) processReading(r types.Reading) error {
	g.ClientChanMutex.RLock()
	defer g.ClientChanMutex.RUnlock()

	for _, v := range g.ClientChans {
		select {
		case v <- r:
		default:
			log.Debugf("gRPC client channel full, dropping reading")
		}
	}

	log.Debugf("gRPC distributed reading to %d clients", len(g.ClientChans))
	return nil
}

func (g *Storage) CheckHealth(configProvider config.ConfigProvider) *config.StorageHealthData {
	if g.Server == nil {
		return storage.CreateHealthData("unhealthy", "gRPC server not initialized", errors.New("server instance is nil"))
	}

	var details []string
	details = append(details, "server: running")

	if g.GRPCConfig != nil {
		details = append(details, fmt.Sprintf("port %d: configured", g.GRPCConfig.Port))
	}

	if g.DBEnabled && g.DBClient != nil {
		if g.DBClient.DB == nil {
			return storage.CreateHealthData("unhealthy", "Database client not connected", errors.New("DB client connection is nil"))
		}

		sqlDB, err := g.DBClient.DB.DB()
		if err != nil {
			return storage.CreateHealthData("unhealthy", "Failed to get underlying database connection", err)
		}

		if err := sqlDB.Ping(); err != nil {
			return storage.CreateHealthData("unhealthy", "Database ping failed", err)
		}

		details = append(details, "database: connected")
	} else {
		details = append(details, "database: disabled")
	}

	g.ClientChanMutex.RLock()
	clientCount := len(g.ClientChans)
	g.ClientChanMutex.RUnlock()
	details = append(details, fmt.Sprintf("clients: %d connected", clientCount))

	return storage.CreateHealthData("healthy", fmt.Sprintf("gRPC server operational (%s)", strings.Join(details, ", ")), nil)
}

// New sets up a new gRPC storage backend
func New(ctx context.Context, configProvider config.ConfigProvider) (*Storage, error) {
	var err error
	var g Storage

	// Load configuration
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		return &Storage{}, fmt.Errorf("error loading configuration: %v", err)
	}

	if cfgData.Storage.GRPC == nil {
		return &Storage{}, fmt.Errorf("GRPC storage configuration is missing")
	}

	grpcConfig := cfgData.Storage.GRPC

	if grpcConfig.Cert != "" && grpcConfig.Key != "" {
		// Create the TLS credentials
		creds, err := credentials.NewServerTLSFromFile(grpcConfig.Cert, grpcConfig.Key)
		if err != nil {
			return &Storage{}, fmt.Errorf("could not create TLS server from keypair: %v", err)
		}
		g.Server = grpc.NewServer(grpc.Creds(creds))
	} else {
		g.Server = grpc.NewServer()
	}

	if grpcConfig.PullFromDevice == "" {
		return &Storage{}, errors.New("you must configure a pull-from-device to specify the default station to pull data for")
	}

	// Store a reference to our configuration in our Storage object
	g.GRPCConfig = grpcConfig

	// Create device manager using shared utility
	g.DeviceManager = grpcutil.NewDeviceManager(cfgData.Devices)

	// Optionally, add gRPC reflection to our servers so that clients can self-discover
	// our methods.
	reflection.Register(g.Server)

	listenAddr := fmt.Sprintf(":%v", grpcConfig.Port)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return &Storage{}, fmt.Errorf("could not create gRPC listener: %v", err)
	}

	// If a TimescaleDB database was configured, create a database client
	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.GetConnectionString() != "" {
		g.DBClient = database.NewClient(configProvider, log.GetZapLogger().Sugar())
		err = g.DBClient.Connect()
		if err != nil {
			return &Storage{}, fmt.Errorf("gRPC storage could not connect to database: %v", err)
		}
		g.DBEnabled = true
	}

	weather.RegisterWeatherV1Server(g.Server, &g)
	go g.Server.Serve(l)

	// Start health monitoring
	storage.StartHealthMonitor(ctx, configProvider, "grpc", &g, 60*time.Second)

	return &g, nil
}

// registerClient creates a channel for sending readings to a client and adds it
// to the slice of active client channels
func (g *Storage) registerClient(clientChan chan types.Reading) int {
	g.ClientChanMutex.Lock()
	defer g.ClientChanMutex.Unlock()

	g.ClientChans = append(g.ClientChans, clientChan)
	return len(g.ClientChans) - 1
}

func (g *Storage) deregisterClient(i int) {
	g.ClientChanMutex.Lock()
	defer g.ClientChanMutex.Unlock()

	g.ClientChans[i] = g.ClientChans[len(g.ClientChans)-1]
	g.ClientChans = g.ClientChans[:len(g.ClientChans)-1]
}

func (g *Storage) GetWeatherSpan(ctx context.Context, request *weather.WeatherSpanRequest) (*weather.WeatherSpan, error) {
	if !g.DBEnabled {
		return nil, fmt.Errorf("database not configured")
	}

	var dbFetchedReadings []types.BucketReading
	spanStart := time.Now().Add(-request.SpanDuration.AsDuration())

	// Validate station name using shared utility
	if err := grpcutil.ValidateStationRequest(request.StationName, g.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := g.DeviceManager.GetSnowBaseDistance(request.StationName)

	// Query database with station filter and snow depth calculation
	g.DBClient.DB.Table("weather_1m").
		Select("*, (? - snowdistance) AS snowdepth", baseDistance).
		Where("bucket > ?", spanStart).
		Where("stationname = ?", request.StationName).
		Find(&dbFetchedReadings)

	log.Debugf("GetWeatherSpan returned %d rows for station %s, span %v", len(dbFetchedReadings), request.StationName, request.SpanDuration.AsDuration())

	span := &weather.WeatherSpan{
		SpanStart: timestamppb.New(spanStart),
		Reading:   grpcutil.TransformBucketReadings(&dbFetchedReadings),
	}

	return span, nil
}

func (g *Storage) GetLatestReading(ctx context.Context, request *weather.LatestReadingRequest) (*weather.WeatherReading, error) {
	if !g.DBEnabled {
		return nil, fmt.Errorf("database not configured")
	}

	var dbFetchedReadings []types.BucketReading

	// Validate station name using shared utility
	if err := grpcutil.ValidateStationRequest(request.StationName, g.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for the specified station
	baseDistance := g.DeviceManager.GetSnowBaseDistance(request.StationName)

	// Query database with station filter and snow depth calculation
	g.DBClient.DB.Table("weather_1m").
		Select("*, (? - snowdistance) AS snowdepth", baseDistance).
		Where("stationname = ?", request.StationName).
		Order("bucket desc").
		Limit(1).
		Find(&dbFetchedReadings)

	if len(dbFetchedReadings) == 0 {
		return nil, fmt.Errorf("no weather readings found for station: %s", request.StationName)
	}

	readings := grpcutil.TransformBucketReadings(&dbFetchedReadings)
	if len(readings) > 0 {
		return readings[0], nil
	}

	return nil, fmt.Errorf("no weather readings found for station: %s", request.StationName)
}

// GetLiveWeather implements the live weather feed for WeatherServer
func (g *Storage) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.WeatherV1_GetLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	// Validate station name using shared utility
	if err := grpcutil.ValidateStationRequest(req.StationName, g.DeviceManager); err != nil {
		return err
	}

	log.Infof("Registering new gRPC streaming client [%v] for station [%s]...", p.Addr, req.StationName)
	clientChan := make(chan types.Reading, 10)
	clientIndex := g.registerClient(clientChan)
	defer g.deregisterClient(clientIndex)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			r := <-clientChan

			// Only send the reading if it matches the requested station
			if r.StationName == req.StationName {
				log.Debugf("Sending reading to client [%v]", p.Addr)

				// Transform reading using shared utility
				grpcReading := grpcutil.TransformReading(r)
				stream.Send(grpcReading)
			}
		}
	}
}
