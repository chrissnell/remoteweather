package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	weather "github.com/chrissnell/gopherwx/protobuf"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCConfig describes the YAML-provided configuration for a gRPC
// storage backend
type GRPCConfig struct {
	TLS  bool `yaml:"use-tls"`
	Port int  `yaml:"port,omitempty"`
}

// GRPCStorage implements a gRPC storage backend
type GRPCStorage struct {
	GRPCServer     *grpc.Server
	Listener       net.Listener
	RPCReadingChan chan Reading
	Ctx            context.Context
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (g GRPCStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Println("Starting gRPC storage engine...")
	g.Ctx = ctx
	readingChan := make(chan Reading)
	go g.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (g GRPCStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			err := g.SendReading(r)
			if err != nil {
				log.Println(err)
			}
		case <-ctx.Done():
			log.Println("Cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// SendReading sends a reading value to our gRPC clients
func (g GRPCStorage) SendReading(r Reading) error {
	select {
	case g.RPCReadingChan <- r:
	default:
	}

	return nil
}

// NewGRPCStorage sets up a new gRPC storage backend
func NewGRPCStorage(c *Config) (GRPCStorage, error) {
	var err error
	g := GRPCStorage{}

	g.RPCReadingChan = make(chan Reading, 10)

	listenAddr := fmt.Sprintf(":%v", c.Storage.GRPC.Port)

	g.Listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return GRPCStorage{}, fmt.Errorf("Could not create gRPC listener: %v", err)
	}

	g.GRPCServer = grpc.NewServer()
	weather.RegisterWeatherServer(g.GRPCServer, &g)
	reflection.Register(g.GRPCServer)
	go g.GRPCServer.Serve(g.Listener)

	return g, nil
}

// GetLiveWeather satisfies the implementation of weather.WeatherServer
func (g *GRPCStorage) GetLiveWeather(e *weather.Empty, stream weather.Weather_GetLiveWeatherServer) error {
	log.Println("Starting GetLiveWeather()...")
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			r := <-g.RPCReadingChan
			rts, _ := ptypes.TimestampProto(r.Timestamp)

			stream.Send(&weather.WeatherReading{
				LastReading:     rts,
				OutsideTemp:     r.OutTemp,
				OutsideHumidity: int32(r.OutHumidity),
				Barometer:       r.Barometer,
				WindSpeed:       int32(r.WindSpeed),
				WindDir:         int32(r.WindDir),
				RainfallDay:     r.DayRain,
			})

		}
	}
}
