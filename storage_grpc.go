package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	weather "github.com/chrissnell/gopherwx/protobuf"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GRPCConfig describes the YAML-provided configuration for a gRPC
// storage backend
type GRPCConfig struct {
	Cert           string `yaml:"cert,omitempty"`
	Key            string `yaml:"key,omitempty"`
	ListenAddr     string `yaml:"listen-addr,omitempty"`
	Port           int    `yaml:"port,omitempty"`
	PullFromDevice string `yaml:"pull-from-device,omitempty"`
}

// GRPCStorage implements a gRPC storage backend
type GRPCStorage struct {
	ClientChans     []chan Reading
	ClientChanMutex sync.RWMutex
	DB              *gorm.DB
	DBEnabled       bool
	Server          *grpc.Server
	GRPCConfig      *GRPCConfig

	weather.UnimplementedWeatherServer
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (g *GRPCStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting gRPC storage engine...")
	readingChan := make(chan Reading)
	go g.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (g *GRPCStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			g.ClientChanMutex.RLock()
			// Send the Reading we just received to all client channels.
			// If there are no clients connected, it gets discarded.
			for _, v := range g.ClientChans {
				v <- r
			}
			g.ClientChanMutex.RUnlock()
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			g.Server.Stop()
			return
		}
	}
}

// NewGRPCStorage sets up a new gRPC storage backend
func NewGRPCStorage(ctx context.Context, c *Config) (*GRPCStorage, error) {
	var err error
	var g GRPCStorage

	if c.Storage.GRPC.Cert != "" && c.Storage.GRPC.Key != "" {
		// Create the TLS credentials
		creds, err := credentials.NewServerTLSFromFile(c.Storage.GRPC.Cert, c.Storage.GRPC.Key)
		if err != nil {
			return &GRPCStorage{}, fmt.Errorf("could not create TLS server from keypair: %v", err)
		}
		g.Server = grpc.NewServer(grpc.Creds(creds))
	} else {
		g.Server = grpc.NewServer()
	}

	if c.Storage.GRPC.PullFromDevice == "" {
		return &GRPCStorage{}, errors.New("you must configure a pull-from-device to specify the default station to pull data for")
	}

	// Store a reference to our configuration in our GRPCStorage object
	g.GRPCConfig = &c.Storage.GRPC

	// Optionally, add gRPC reflection to our servers so that clients can self-discover
	// our methods.
	reflection.Register(g.Server)

	listenAddr := fmt.Sprintf(":%v", c.Storage.GRPC.Port)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return &GRPCStorage{}, fmt.Errorf("could not create gRPC listener: %v", err)
	}

	// If a TimescaleDB database was configured, set up a GORM DB handle so that the
	// gRPC handlers can retrieve data
	if c.Storage.TimescaleDB.ConnectionString != "" {
		err = g.connectToDatabase(c.Storage.TimescaleDB.ConnectionString)
		if err != nil {
			return &GRPCStorage{}, fmt.Errorf("gRPC storage could not connect to database: %v", err)
		}
		g.DBEnabled = true
	}

	weather.RegisterWeatherServer(g.Server, &g)
	go g.Server.Serve(l)

	return &g, nil
}

func (g *GRPCStorage) connectToDatabase(dbURI string) error {
	var err error
	// Create a logger for gorm
	dbLogger := logger.New(
		zap.NewStdLog(zapLogger),
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	log.Info("connecting to TimescaleDB for gRPC data backend...")
	g.DB, err = gorm.Open(postgres.Open(dbURI), &gorm.Config{Logger: dbLogger})
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}

	return nil
}

// registerClient creates a channel for sending readings to a client and adds it
// to the slice of active client channels
func (g *GRPCStorage) registerClient(clientChan chan Reading) int {
	g.ClientChanMutex.Lock()
	defer g.ClientChanMutex.Unlock()

	g.ClientChans = append(g.ClientChans, clientChan)
	return len(g.ClientChans) - 1
}

func (g *GRPCStorage) deregisterClient(i int) {
	g.ClientChanMutex.Lock()
	defer g.ClientChanMutex.Unlock()

	g.ClientChans[i] = g.ClientChans[len(g.ClientChans)-1]
	g.ClientChans = g.ClientChans[:len(g.ClientChans)-1]
}

func (g *GRPCStorage) GetWeatherSpan(ctx context.Context, request *weather.WeatherSpanRequest) (*weather.WeatherSpan, error) {

	var dbFetchedReadings []BucketReading

	spanStart := time.Now().Add(-request.SpanDuration.AsDuration())

	if g.DBEnabled {
		g.DB.Table("weather_1m").Where("bucket > ?", spanStart).Find(&dbFetchedReadings)
		log.Infof("returned rows: %v", len(dbFetchedReadings))

		span := &weather.WeatherSpan{
			SpanStart: (*timestamppb.Timestamp)(timestamppb.New(spanStart)),
			Reading:   g.transformReadings(&dbFetchedReadings)}

		log.Infof("getweatherspan -> spanDuration: %v", request.SpanDuration.AsDuration())

		return span, nil
	}

	return &weather.WeatherSpan{}, fmt.Errorf("ignoring GetWeatherSpan request: database not configured")
}

func (g *GRPCStorage) transformReadings(dbReadings *[]BucketReading) []*weather.WeatherReading {
	grpcReadings := make([]*weather.WeatherReading, 0)

	for _, r := range *dbReadings {
		grpcReadings = append(grpcReadings, &weather.WeatherReading{
			ReadingTimestamp:   (*timestamppb.Timestamp)(timestamppb.New(r.Bucket)),
			OutsideTemperature: r.OutTemp,
			OutsideHumidity:    int32(r.OutHumidity),
			Barometer:          r.Barometer,
			WindSpeed:          int32(r.WindSpeed),
			WindDirection:      int32(r.WindDir),
			RainfallDay:        r.DayRain,
			WindChill:          r.WindChill,
			HeatIndex:          r.HeatIndex,
			InsideTemperature:  r.InTemp,
			InsideHumidity:     int32(r.InHumidity),
		})
	}

	return grpcReadings
}

// GetLiveWeather implements the live weather feed for WeatherServer
func (g *GRPCStorage) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.Weather_GetLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	log.Infof("Registering new gRPC streaming client [%v]...", p.Addr)
	clientChan := make(chan Reading, 10)
	clientIndex := g.registerClient(clientChan)
	defer g.deregisterClient(clientIndex)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			r := <-clientChan

			// Only send the reading if the station name matches the PullFromDevice set in the config,
			// or if it matches the StationName in the request
			if (r.StationName == g.GRPCConfig.PullFromDevice) || (req.StationName != nil && r.StationName == *req.StationName) {

				log.Debugf("Sending reading to client [%v]", p.Addr)

				//rts, _ := ptypes.TimestampProto(r.Timestamp)
				rts := timestamppb.New(r.Timestamp)

				stream.Send(&weather.WeatherReading{
					ReadingTimestamp:   rts,
					OutsideTemperature: r.OutTemp,
					InsideTemperature:  r.InTemp,
					OutsideHumidity:    int32(r.OutHumidity),
					InsideHumidity:     int32(r.InHumidity),
					Barometer:          r.Barometer,
					WindSpeed:          int32(r.WindSpeed),
					WindDirection:      int32(r.WindDir),
					RainfallDay:        r.DayRain,
					StationName:        r.StationName,
				})
			}

		}
	}
}
