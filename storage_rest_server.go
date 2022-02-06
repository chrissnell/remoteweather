package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// RESTServerConfig describes the YAML-provided configuration for a REST
// server storage backend
type RESTServerConfig struct {
	Cert       string `yaml:"cert,omitempty"`
	Key        string `yaml:"key,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	ListenAddr string `yaml:"listen_addr,omitempty"`
}

// RESTServerStorage implements a REST server storage backend
type RESTServerStorage struct {
	ClientChans     []chan Reading
	ClientChanMutex sync.RWMutex
	Server          http.Server
	DB              *gorm.DB
	DBEnabled       bool
}

type RESTWeatherReading struct {
	ReadingTimestamp time.Time `json:"ts"`
	// Using pointers for readings ensures that json.Marshall will encode zeros as 0
	// instead of simply not including the field in the data structure
	OutsideTemperature *float32 `json:"otemp"`
	OutsideHumidity    *float32 `json:"ohum"`
	Barometer          *float32 `json:"bar"`
	WindSpeed          *float32 `json:"winds"`
	WindDirection      *float32 `json:"windd"`
	RainfallDay        *float32 `json:"rainday"`
	WindChill          *float32 `json:"windch"`
	HeatIndex          *float32 `json:"heatidx"`
	InsideTemperature  *float32 `json:"itemp"`
	InsideHumidity     *float32 `json:"ihum"`
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to our gRPC clients
func (r *RESTServerStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- Reading {
	log.Info("starting REST server storage engine...")
	readingChan := make(chan Reading)
	go r.processMetrics(ctx, wg, readingChan)
	return readingChan
}

func (r *RESTServerStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case reading := <-rchan:
			r.ClientChanMutex.RLock()
			// Send the Reading we just received to all client channels.
			// If there are no clients connected, it gets discarded.
			for _, v := range r.ClientChans {
				v <- reading
			}
			r.ClientChanMutex.RUnlock()
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling readings processor.")
			r.Server.Shutdown(context.Background())
			return
		}
	}
}

// NewRESTServerStorage sets up a new REST server storage backend
func NewRESTServerStorage(ctx context.Context, c *Config) (*RESTServerStorage, error) {
	var err error

	r := new(RESTServerStorage)

	// If a ListenAddr was not provided, listen on all interfaces
	if c.Storage.RESTServer.ListenAddr == "" {
		log.Info("rest.listen_addr not provided; defaulting to 0.0.0.0 (all interfaces)")
		c.Storage.RESTServer.ListenAddr = "0.0.0.0"
	}

	router := mux.NewRouter()
	router.HandleFunc("/span/{span}", r.getWeatherSpan)

	r.Server.Addr = fmt.Sprintf("%v:%v", c.Storage.RESTServer.ListenAddr, c.Storage.RESTServer.Port)

	if c.Storage.RESTServer.Cert != "" && c.Storage.RESTServer.Key != "" {
		go r.Server.ListenAndServeTLS(c.Storage.RESTServer.Cert, c.Storage.RESTServer.Key)
	} else {
		go r.Server.ListenAndServe()
	}

	// Configure our mux router as the handler for our Server
	r.Server.Handler = router

	// If a TimescaleDB database was configured, set up a GORM DB handle so that the
	// handlers can retrieve data
	if c.Storage.TimescaleDB.ConnectionString != "" {
		err = r.connectToDatabase(c.Storage.TimescaleDB.ConnectionString)
		if err != nil {
			return &RESTServerStorage{}, fmt.Errorf("gRPC storage could not connect to database: %v", err)
		}
		r.DBEnabled = true
	}

	return r, nil
}

func (r *RESTServerStorage) connectToDatabase(dbURI string) error {
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
	r.DB, err = gorm.Open(postgres.Open(dbURI), &gorm.Config{Logger: dbLogger})
	if err != nil {
		log.Warn("warning: unable to create a TimescaleDB connection:", err)
		return err
	}

	return nil
}

func (r *RESTServerStorage) getWeatherSpan(w http.ResponseWriter, req *http.Request) {

	var dbFetchedReadings []BucketReading

	vars := mux.Vars(req)
	span, err := time.ParseDuration(vars["span"])
	if err != nil {
		log.Errorf("invalid request: unable to parse duration: %v", vars["span"])
		http.Error(w, "error: invalid span duration", 400)
		return
	}

	spanStart := time.Now().Add(-span)

	if r.DBEnabled {
		r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Find(&dbFetchedReadings)
		log.Infof("returned rows: %v", len(dbFetchedReadings))

		log.Infof("getweatherspan -> spanDuration: %v", span)

		err := json.NewEncoder(w).Encode(r.transformReadings(&dbFetchedReadings))
		if err != nil {
			log.Errorf("error marshalling dbFetchedReadings: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
	}
}

func (r *RESTServerStorage) transformReadings(dbReadings *[]BucketReading) []*RESTWeatherReading {
	wr := make([]*RESTWeatherReading, 0)

	for _, r := range *dbReadings {
		wr = append(wr, &RESTWeatherReading{
			ReadingTimestamp:   r.Bucket,
			OutsideTemperature: &r.OutTemp,
			OutsideHumidity:    &r.OutHumidity,
			Barometer:          &r.Barometer,
			WindSpeed:          &r.WindSpeed,
			WindDirection:      &r.WindDir,
			RainfallDay:        &r.DayRain,
			WindChill:          &r.Windchill,
			HeatIndex:          &r.HeatIndex,
			InsideTemperature:  &r.InTemp,
			InsideHumidity:     &r.InHumidity,
		})
	}

	return wr
}

// // GetLiveWeather implements the live weather feed for WeatherServer
// func (r *RESTServerStorage) GetLiveWeather(e *weather.Empty, stream weather.Weather_GetLiveWeatherServer) error {
// 	ctx := stream.Context()
// 	p, _ := peer.FromContext(ctx)

// 	log.Infof("Registering new gRPC streaming client [%v]...", p.Addr)
// 	clientChan := make(chan Reading, 10)
// 	clientIndex := r.registerClient(clientChan)

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Infof("Deregistering gRPC streaming client [%v:%v]", clientIndex, p.Addr)
// 			r.deregisterClient(clientIndex)
// 			return nil
// 		default:
// 			r := <-clientChan
// 			log.Debugf("Sending reading to client [%v]", p.Addr)

// 			//rts, _ := ptypes.TimestampProto(r.Timestamp)
// 			rts := timestamppb.New(r.Timestamp)

// 			stream.Send(&weather.WeatherReading{
// 				ReadingTimestamp:   rts,
// 				OutsideTemperature: r.OutTemp,
// 				InsideTemperature:  r.InTemp,
// 				OutsideHumidity:    float32(r.OutHumidity),
// 				InsideHumidity:     float32(r.InHumidity),
// 				Barometer:          r.Barometer,
// 				WindSpeed:          float32(r.WindSpeed),
// 				WindDirection:      float32(r.WindDir),
// 				RainfallDay:        r.DayRain,
// 			})

// 		}
// 	}
// }
