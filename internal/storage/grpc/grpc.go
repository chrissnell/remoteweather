// Package grpc provides gRPC storage backend for streaming weather data to remote clients.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
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

func (g *Storage) GetLatestReading(ctx context.Context, request *weather.LatestReadingRequest) (*weather.WeatherReading, error) {
	var dbFetchedReadings []types.BucketReading

	if g.DBEnabled {
		query := g.DBClient.DB.Table("weather_1m").Order("bucket desc").Limit(1)

		// Filter by station name if provided
		if request.StationName != nil && *request.StationName != "" {
			query = query.Where("stationname = ?", *request.StationName)
		}

		query.Find(&dbFetchedReadings)

		if len(dbFetchedReadings) == 0 {
			return &weather.WeatherReading{}, fmt.Errorf("no weather readings found")
		}

		readings := g.transformReadings(&dbFetchedReadings)
		if len(readings) > 0 {
			return readings[0], nil
		}
	}

	return &weather.WeatherReading{}, fmt.Errorf("ignoring GetLatestReading request: database not configured")
}

func (g *Storage) transformReadings(dbReadings *[]types.BucketReading) []*weather.WeatherReading {
	// Pre-allocate slice with exact capacity to avoid multiple reallocations
	grpcReadings := make([]*weather.WeatherReading, 0, len(*dbReadings))

	for _, r := range *dbReadings {
		grpcReadings = append(grpcReadings, &weather.WeatherReading{
			ReadingTimestamp: (*timestamppb.Timestamp)(timestamppb.New(r.Bucket)),
			StationName:      r.StationName,
			StationType:      r.StationType,

			// Primary environmental readings
			Barometer:          r.Barometer,
			InsideTemperature:  r.InTemp,
			InsideHumidity:     r.InHumidity,
			OutsideTemperature: r.OutTemp,
			OutsideHumidity:    r.OutHumidity,

			// Wind measurements
			WindSpeed:     r.WindSpeed,
			WindSpeed10:   r.WindSpeed10,
			WindDirection: r.WindDir,
			WindChill:     r.WindChill,
			HeatIndex:     r.HeatIndex,

			// Additional temperature sensors
			ExtraTemp1: r.ExtraTemp1,
			ExtraTemp2: r.ExtraTemp2,
			ExtraTemp3: r.ExtraTemp3,
			ExtraTemp4: r.ExtraTemp4,
			ExtraTemp5: r.ExtraTemp5,
			ExtraTemp6: r.ExtraTemp6,
			ExtraTemp7: r.ExtraTemp7,

			// Soil temperature sensors
			SoilTemp1: r.SoilTemp1,
			SoilTemp2: r.SoilTemp2,
			SoilTemp3: r.SoilTemp3,
			SoilTemp4: r.SoilTemp4,

			// Leaf temperature sensors
			LeafTemp1: r.LeafTemp1,
			LeafTemp2: r.LeafTemp2,
			LeafTemp3: r.LeafTemp3,
			LeafTemp4: r.LeafTemp4,

			// Additional humidity sensors
			ExtraHumidity1: r.ExtraHumidity1,
			ExtraHumidity2: r.ExtraHumidity2,
			ExtraHumidity3: r.ExtraHumidity3,
			ExtraHumidity4: r.ExtraHumidity4,
			ExtraHumidity5: r.ExtraHumidity5,
			ExtraHumidity6: r.ExtraHumidity6,
			ExtraHumidity7: r.ExtraHumidity7,

			// Rain measurements
			RainRate:        r.RainRate,
			RainIncremental: r.RainIncremental,
			StormRain:       r.StormRain,
			StormStart:      (*timestamppb.Timestamp)(timestamppb.New(r.StormStart)),
			DayRain:         r.DayRain,
			MonthRain:       r.MonthRain,
			YearRain:        r.YearRain,

			// Solar measurements
			SolarWatts:          r.SolarWatts,
			PotentialSolarWatts: r.PotentialSolarWatts,
			SolarJoules:         r.SolarJoules,
			Uv:                  r.UV,
			Radiation:           r.Radiation,

			// Evapotranspiration
			DayET:   r.DayET,
			MonthET: r.MonthET,
			YearET:  r.YearET,

			// Soil moisture sensors
			SoilMoisture1: r.SoilMoisture1,
			SoilMoisture2: r.SoilMoisture2,
			SoilMoisture3: r.SoilMoisture3,
			SoilMoisture4: r.SoilMoisture4,

			// Leaf wetness sensors
			LeafWetness1: r.LeafWetness1,
			LeafWetness2: r.LeafWetness2,
			LeafWetness3: r.LeafWetness3,
			LeafWetness4: r.LeafWetness4,

			// Alarm states
			InsideAlarm:    uint32(r.InsideAlarm),
			RainAlarm:      uint32(r.RainAlarm),
			OutsideAlarm1:  uint32(r.OutsideAlarm1),
			OutsideAlarm2:  uint32(r.OutsideAlarm2),
			ExtraAlarm1:    uint32(r.ExtraAlarm1),
			ExtraAlarm2:    uint32(r.ExtraAlarm2),
			ExtraAlarm3:    uint32(r.ExtraAlarm3),
			ExtraAlarm4:    uint32(r.ExtraAlarm4),
			ExtraAlarm5:    uint32(r.ExtraAlarm5),
			ExtraAlarm6:    uint32(r.ExtraAlarm6),
			ExtraAlarm7:    uint32(r.ExtraAlarm7),
			ExtraAlarm8:    uint32(r.ExtraAlarm8),
			SoilLeafAlarm1: uint32(r.SoilLeafAlarm1),
			SoilLeafAlarm2: uint32(r.SoilLeafAlarm2),
			SoilLeafAlarm3: uint32(r.SoilLeafAlarm3),
			SoilLeafAlarm4: uint32(r.SoilLeafAlarm4),

			// Battery and power status
			TxBatteryStatus:       uint32(r.TxBatteryStatus),
			ConsBatteryVoltage:    r.ConsBatteryVoltage,
			StationBatteryVoltage: r.StationBatteryVoltage,

			// Forecast information
			ForecastIcon: uint32(r.ForecastIcon),
			ForecastRule: uint32(r.ForecastRule),

			// Astronomical data
			Sunrise: (*timestamppb.Timestamp)(timestamppb.New(r.Sunrise)),
			Sunset:  (*timestamppb.Timestamp)(timestamppb.New(r.Sunset)),

			// Snow measurements
			SnowDistance: r.SnowDistance,
			SnowDepth:    r.SnowDepth,

			// Extended float fields
			ExtraFloat1:  r.ExtraFloat1,
			ExtraFloat2:  r.ExtraFloat2,
			ExtraFloat3:  r.ExtraFloat3,
			ExtraFloat4:  r.ExtraFloat4,
			ExtraFloat5:  r.ExtraFloat5,
			ExtraFloat6:  r.ExtraFloat6,
			ExtraFloat7:  r.ExtraFloat7,
			ExtraFloat8:  r.ExtraFloat8,
			ExtraFloat9:  r.ExtraFloat9,
			ExtraFloat10: r.ExtraFloat10,

			// Extended text fields
			ExtraText1:  r.ExtraText1,
			ExtraText2:  r.ExtraText2,
			ExtraText3:  r.ExtraText3,
			ExtraText4:  r.ExtraText4,
			ExtraText5:  r.ExtraText5,
			ExtraText6:  r.ExtraText6,
			ExtraText7:  r.ExtraText7,
			ExtraText8:  r.ExtraText8,
			ExtraText9:  r.ExtraText9,
			ExtraText10: r.ExtraText10,
		})
	}

	return grpcReadings
}

// GetLiveWeather implements the live weather feed for WeatherServer
func (g *Storage) GetLiveWeather(req *weather.LiveWeatherRequest, stream weather.WeatherV1_GetLiveWeatherServer) error {
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

				stream.Send(&weather.WeatherReading{
					ReadingTimestamp: (*timestamppb.Timestamp)(timestamppb.New(r.Timestamp)),
					StationName:      r.StationName,
					StationType:      r.StationType,

					// Primary environmental readings
					Barometer:          r.Barometer,
					InsideTemperature:  r.InTemp,
					InsideHumidity:     r.InHumidity,
					OutsideTemperature: r.OutTemp,
					OutsideHumidity:    r.OutHumidity,

					// Wind measurements
					WindSpeed:     r.WindSpeed,
					WindSpeed10:   r.WindSpeed10,
					WindDirection: r.WindDir,
					WindChill:     r.WindChill,
					HeatIndex:     r.HeatIndex,

					// Additional temperature sensors
					ExtraTemp1: r.ExtraTemp1,
					ExtraTemp2: r.ExtraTemp2,
					ExtraTemp3: r.ExtraTemp3,
					ExtraTemp4: r.ExtraTemp4,
					ExtraTemp5: r.ExtraTemp5,
					ExtraTemp6: r.ExtraTemp6,
					ExtraTemp7: r.ExtraTemp7,

					// Soil temperature sensors
					SoilTemp1: r.SoilTemp1,
					SoilTemp2: r.SoilTemp2,
					SoilTemp3: r.SoilTemp3,
					SoilTemp4: r.SoilTemp4,

					// Leaf temperature sensors
					LeafTemp1: r.LeafTemp1,
					LeafTemp2: r.LeafTemp2,
					LeafTemp3: r.LeafTemp3,
					LeafTemp4: r.LeafTemp4,

					// Additional humidity sensors
					ExtraHumidity1: r.ExtraHumidity1,
					ExtraHumidity2: r.ExtraHumidity2,
					ExtraHumidity3: r.ExtraHumidity3,
					ExtraHumidity4: r.ExtraHumidity4,
					ExtraHumidity5: r.ExtraHumidity5,
					ExtraHumidity6: r.ExtraHumidity6,
					ExtraHumidity7: r.ExtraHumidity7,

					// Rain measurements
					RainRate:        r.RainRate,
					RainIncremental: r.RainIncremental,
					StormRain:       r.StormRain,
					StormStart:      (*timestamppb.Timestamp)(timestamppb.New(r.StormStart)),
					DayRain:         r.DayRain,
					MonthRain:       r.MonthRain,
					YearRain:        r.YearRain,

					// Solar measurements
					SolarWatts:          r.SolarWatts,
					PotentialSolarWatts: r.PotentialSolarWatts,
					SolarJoules:         r.SolarJoules,
					Uv:                  r.UV,
					Radiation:           r.Radiation,

					// Evapotranspiration
					DayET:   r.DayET,
					MonthET: r.MonthET,
					YearET:  r.YearET,

					// Soil moisture sensors
					SoilMoisture1: r.SoilMoisture1,
					SoilMoisture2: r.SoilMoisture2,
					SoilMoisture3: r.SoilMoisture3,
					SoilMoisture4: r.SoilMoisture4,

					// Leaf wetness sensors
					LeafWetness1: r.LeafWetness1,
					LeafWetness2: r.LeafWetness2,
					LeafWetness3: r.LeafWetness3,
					LeafWetness4: r.LeafWetness4,

					// Alarm states
					InsideAlarm:    uint32(r.InsideAlarm),
					RainAlarm:      uint32(r.RainAlarm),
					OutsideAlarm1:  uint32(r.OutsideAlarm1),
					OutsideAlarm2:  uint32(r.OutsideAlarm2),
					ExtraAlarm1:    uint32(r.ExtraAlarm1),
					ExtraAlarm2:    uint32(r.ExtraAlarm2),
					ExtraAlarm3:    uint32(r.ExtraAlarm3),
					ExtraAlarm4:    uint32(r.ExtraAlarm4),
					ExtraAlarm5:    uint32(r.ExtraAlarm5),
					ExtraAlarm6:    uint32(r.ExtraAlarm6),
					ExtraAlarm7:    uint32(r.ExtraAlarm7),
					ExtraAlarm8:    uint32(r.ExtraAlarm8),
					SoilLeafAlarm1: uint32(r.SoilLeafAlarm1),
					SoilLeafAlarm2: uint32(r.SoilLeafAlarm2),
					SoilLeafAlarm3: uint32(r.SoilLeafAlarm3),
					SoilLeafAlarm4: uint32(r.SoilLeafAlarm4),

					// Battery and power status
					TxBatteryStatus:       uint32(r.TxBatteryStatus),
					ConsBatteryVoltage:    r.ConsBatteryVoltage,
					StationBatteryVoltage: r.StationBatteryVoltage,

					// Forecast information
					ForecastIcon: uint32(r.ForecastIcon),
					ForecastRule: uint32(r.ForecastRule),

					// Astronomical data
					Sunrise: (*timestamppb.Timestamp)(timestamppb.New(r.Sunrise)),
					Sunset:  (*timestamppb.Timestamp)(timestamppb.New(r.Sunset)),

					// Snow measurements
					SnowDistance: r.SnowDistance,
					SnowDepth:    r.SnowDepth,

					// Extended float fields
					ExtraFloat1:  r.ExtraFloat1,
					ExtraFloat2:  r.ExtraFloat2,
					ExtraFloat3:  r.ExtraFloat3,
					ExtraFloat4:  r.ExtraFloat4,
					ExtraFloat5:  r.ExtraFloat5,
					ExtraFloat6:  r.ExtraFloat6,
					ExtraFloat7:  r.ExtraFloat7,
					ExtraFloat8:  r.ExtraFloat8,
					ExtraFloat9:  r.ExtraFloat9,
					ExtraFloat10: r.ExtraFloat10,

					// Extended text fields
					ExtraText1:  r.ExtraText1,
					ExtraText2:  r.ExtraText2,
					ExtraText3:  r.ExtraText3,
					ExtraText4:  r.ExtraText4,
					ExtraText5:  r.ExtraText5,
					ExtraText6:  r.ExtraText6,
					ExtraText7:  r.ExtraText7,
					ExtraText8:  r.ExtraText8,
					ExtraText9:  r.ExtraText9,
					ExtraText10: r.ExtraText10,
				})
			}

		}
	}
}
