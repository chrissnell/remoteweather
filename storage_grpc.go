package main

import (
	"context"
	"fmt"
	"net"
	"sync"

	weather "github.com/chrissnell/gopherwx/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCConfig describes the YAML-provided configuration for a gRPC
// storage backend
type GRPCConfig struct {
	Cert string `yaml:"cert,omitempty"`
	Key  string `yaml:"key,omitempty"`
	Port int    `yaml:"port,omitempty"`
}

// GRPCStorage implements a gRPC storage backend
type GRPCStorage struct {
	GRPCServer      *grpc.Server
	Listener        net.Listener
	RPCReadingChan  chan Reading
	Ctx             context.Context
	ClientChans     []chan Reading
	ClientChanMutex sync.RWMutex
	weather.UnimplementedWeatherServer
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (g *GRPCStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting gRPC storage engine...")
	g.Ctx = ctx
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
			for _, v := range g.ClientChans {
				// log.Infof("Sending reading to client #%v", i)
				v <- r
			}
			g.ClientChanMutex.RUnlock()
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			return
		}
	}
}

// NewGRPCStorage sets up a new gRPC storage backend
func NewGRPCStorage(c *Config) (*GRPCStorage, error) {
	var err error
	g := GRPCStorage{}

	g.RPCReadingChan = make(chan Reading, 10)

	if c.Storage.GRPC.Cert != "" && c.Storage.GRPC.Key != "" {
		// Create the TLS credentials
		creds, err := credentials.NewServerTLSFromFile(c.Storage.GRPC.Cert, c.Storage.GRPC.Key)
		if err != nil {
			return &GRPCStorage{}, fmt.Errorf("could not create TLS server from keypair: %v", err)
		}
		g.GRPCServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		g.GRPCServer = grpc.NewServer()
	}

	listenAddr := fmt.Sprintf(":%v", c.Storage.GRPC.Port)

	g.Listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return &GRPCStorage{}, fmt.Errorf("could not create gRPC listener: %v", err)
	}

	weather.RegisterWeatherServer(g.GRPCServer, &g)

	go g.GRPCServer.Serve(g.Listener)

	return &g, nil
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

// GetLiveWeather satisfies the implementation of weather.WeatherServer
func (g *GRPCStorage) GetLiveWeather(e *weather.Empty, stream weather.Weather_GetLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	log.Infof("Registering new gRPC client [%v]...", p.Addr)
	clientChan := make(chan Reading, 10)
	clientIndex := g.registerClient(clientChan)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Deregistering gRPC client [%v:%v]", clientIndex, p.Addr)
			g.deregisterClient(clientIndex)
			return nil
		default:
			r := <-clientChan
			log.Debugf("Sending reading to client [%v]", p.Addr)

			//rts, _ := ptypes.TimestampProto(r.Timestamp)
			rts := timestamppb.New(r.Timestamp)

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
