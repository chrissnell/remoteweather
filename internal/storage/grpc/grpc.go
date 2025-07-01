package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
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

	weather.UnimplementedWeatherServer
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (g *Storage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting gRPC storage engine...")
	readingChan := make(chan types.Reading)
	go g.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (g *Storage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan types.Reading) {
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

	// Optionally, add gRPC reflection to our servers so that clients can self-discover
	// our methods.
	reflection.Register(g.Server)

	listenAddr := fmt.Sprintf(":%v", grpcConfig.Port)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return &Storage{}, fmt.Errorf("could not create gRPC listener: %v", err)
	}

	// If a TimescaleDB database was configured, create a database client
	if cfgData.Storage.TimescaleDB != nil && cfgData.Storage.TimescaleDB.ConnectionString != "" {
		g.DBClient = database.NewClient(configProvider, log.GetZapLogger().Sugar())
		err = g.DBClient.Connect()
		if err != nil {
			return &Storage{}, fmt.Errorf("gRPC storage could not connect to database: %v", err)
		}
		g.DBEnabled = true
	}

	weather.RegisterWeatherServer(g.Server, &g)
	go g.Server.Serve(l)

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

	var dbFetchedReadings []types.BucketReading

	spanStart := time.Now().Add(-request.SpanDuration.AsDuration())

	if g.DBEnabled {
		g.DBClient.DB.Table("weather_1m").Where("bucket > ?", spanStart).Find(&dbFetchedReadings)
		log.Infof("returned rows: %v", len(dbFetchedReadings))

		span := &weather.WeatherSpan{
			SpanStart: (*timestamppb.Timestamp)(timestamppb.New(spanStart)),
			Reading:   g.transformReadings(&dbFetchedReadings)}

		log.Infof("getweatherspan -> spanDuration: %v", request.SpanDuration.AsDuration())

		return span, nil
	}

	return &weather.WeatherSpan{}, fmt.Errorf("ignoring GetWeatherSpan request: database not configured")
}

func (g *Storage) transformReadings(dbReadings *[]types.BucketReading) []*weather.WeatherReading {
	// Pre-allocate slice with exact capacity to avoid multiple reallocations
	grpcReadings := make([]*weather.WeatherReading, 0, len(*dbReadings))

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
func (g *Storage) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.Weather_GetLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	log.Infof("Registering new gRPC streaming client [%v]...", p.Addr)
	clientChan := make(chan types.Reading, 10)
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
