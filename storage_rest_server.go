package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type WeatherSiteConfig struct {
	StationName string `yaml:"station_name,omitempty"`
	PageTitle   string `yaml:"page_title,omitempty"`
}

// RESTServerConfig describes the YAML-provided configuration for a REST
// server storage backend
type RESTServerConfig struct {
	Cert              string            `yaml:"cert,omitempty"`
	Key               string            `yaml:"key,omitempty"`
	Port              int               `yaml:"port,omitempty"`
	ListenAddr        string            `yaml:"listen_addr,omitempty"`
	WeatherSiteConfig WeatherSiteConfig `yaml:"weather_site,omitempty"`
}

// RESTServerStorage implements a REST server storage backend
type RESTServerStorage struct {
	ClientChans       []chan Reading
	ClientChanMutex   sync.RWMutex
	Server            http.Server
	DB                *gorm.DB
	DBEnabled         bool
	FS                *fs.FS
	WeatherSiteConfig *WeatherSiteConfig
}

type WeatherReading struct {
	StationName      string `json:"stationname"`
	ReadingTimestamp int64  `json:"ts"`
	// Using pointers for readings ensures that json.Marshall will encode zeros as 0
	// instead of simply not including the field in the data structure
	OutsideTemperature    float32 `json:"otemp,omitempty"`
	ExtraTemp1            float32 `json:"extratemp1,omitempty"`
	ExtraTemp2            float32 `json:"extratemp2,omitempty"`
	ExtraTemp3            float32 `json:"extratemp3,omitempty"`
	ExtraTemp4            float32 `json:"extratemp4,omitempty"`
	ExtraTemp5            float32 `json:"extratemp5,omitempty"`
	ExtraTemp6            float32 `json:"extratemp6,omitempty"`
	ExtraTemp7            float32 `json:"extratemp7,omitempty"`
	SoilTemp1             float32 `json:"soiltemp1,omitempty"`
	SoilTemp2             float32 `json:"soiltemp2,omitempty"`
	SoilTemp3             float32 `json:"soiltemp3,omitempty"`
	SoilTemp4             float32 `json:"soiltemp4,omitempty"`
	LeafTemp1             float32 `json:"leaftemp1,omitempty"`
	LeafTemp2             float32 `json:"leaftemp2,omitempty"`
	LeafTemp3             float32 `json:"leaftemp3,omitempty"`
	LeafTemp4             float32 `json:"leaftemp4,omitempty"`
	OutHumidity           float32 `json:"outhumidity,omitempty"`
	ExtraHumidity1        float32 `json:"extrahumidity1,omitempty"`
	ExtraHumidity2        float32 `json:"extrahumidity2,omitempty"`
	ExtraHumidity3        float32 `json:"extrahumidity3,omitempty"`
	ExtraHumidity4        float32 `json:"extrahumidity4,omitempty"`
	ExtraHumidity5        float32 `json:"extrahumidity5,omitempty"`
	ExtraHumidity6        float32 `json:"extrahumidity6,omitempty"`
	ExtraHumidity7        float32 `json:"extrahumidity7,omitempty"`
	OutsideHumidity       float32 `json:"ohum,omitempty"`
	RainRate              float32 `json:"rainrate,omitempty"`
	RainIncremental       float32 `json:"rainincremental,omitempty"`
	SolarWatts            float32 `json:"solarwatts,omitempty"`
	SolarJoules           float32 `json:"solarjoules,omitempty"`
	UV                    float32 `json:"uv,omitempty"`
	Radiation             float32 `json:"radiation,omitempty"`
	StormRain             float32 `json:"stormrain,omitempty"`
	DayRain               float32 `json:"dayrain,omitempty"`
	MonthRain             float32 `json:"monthrain,omitempty"`
	YearRain              float32 `json:"yearrain,omitempty"`
	Barometer             float32 `json:"bar,omitempty"`
	WindSpeed             float32 `json:"winds,omitempty"`
	WindDirection         float32 `json:"windd,omitempty"`
	RainfallDay           float32 `json:"rainday,omitempty"`
	WindChill             float32 `json:"windch,omitempty"`
	HeatIndex             float32 `json:"heatidx,omitempty"`
	InsideTemperature     float32 `json:"itemp,omitempty"`
	InsideHumidity        float32 `json:"ihum,omitempty"`
	ConsBatteryVoltage    float32 `json:"consbatteryvoltage,omitempty"`
	StationBatteryVoltage float32 `json:"stationbatteryvoltage,omitempty"`
}

const (
	Day   = 24 * time.Hour
	Month = Day * 30
)

var (
	//go:embed all:assets
	content embed.FS
)

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

	if c.Storage.RESTServer.WeatherSiteConfig.StationName != "" {
		r.WeatherSiteConfig = &c.Storage.RESTServer.WeatherSiteConfig
	}

	fs, _ := fs.Sub(fs.FS(content), "assets")
	r.FS = &fs

	router := mux.NewRouter()
	router.HandleFunc("/span/{span}", r.getWeatherSpan)
	router.HandleFunc("/latest", r.getWeatherLatest)

	router.HandleFunc("/", r.serveIndexTemplate)
	router.HandleFunc("/index.html.tmpl", r.serveIndexTemplate)
	router.PathPrefix("/").Handler(http.FileServer(http.FS(*r.FS)))
	// works
	// router.NotFoundHandler = http.FileServer(http.FS(*r.FS))

	// works
	// router.HandleFunc("/", r.serveIndexTemplate)
	// router.PathPrefix("/").Handler(http.FileServer(http.FS(*r.FS)))

	r.Server.Addr = fmt.Sprintf("%v:%v", c.Storage.RESTServer.ListenAddr, c.Storage.RESTServer.Port)

	if c.Storage.RESTServer.Cert != "" && c.Storage.RESTServer.Key != "" {
		go r.Server.ListenAndServeTLS(c.Storage.RESTServer.Cert, c.Storage.RESTServer.Key)
	} else {
		go r.Server.ListenAndServe()
	}

	go func() {
		<-ctx.Done()
		fmt.Println("Shutting down the HTTP server...")
		r.Server.Shutdown(ctx)
	}()

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

func (r *RESTServerStorage) serveIndexTemplate(w http.ResponseWriter, req *http.Request) {
	view := template.Must(template.New("index.html.tmpl").ParseFS(*r.FS, "index.html.tmpl"))

	w.Header().Set("Content-Type", "text/html")
	err := view.Execute(w, r.WeatherSiteConfig)
	if err != nil {
		log.Error("error executing template:", err)
		return
	}
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

	if r.DBEnabled {
		var dbFetchedReadings []BucketReading

		stationName := req.URL.Query().Get("station")

		vars := mux.Vars(req)
		span, err := time.ParseDuration(vars["span"])
		if err != nil {
			log.Errorf("invalid request: unable to parse duration: %v", vars["span"])
			http.Error(w, "error: invalid span duration", 400)
			return
		}

		spanStart := time.Now().Add(-span)

		// switch {
		// case span < 2*Day:
		// 	r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
		// case (span >= 2*Day) && (span <= 2*Month):
		// 	r.DB.Table("weather_5m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
		// default:
		// 	r.DB.Table("weather_1h").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
		// }

		switch {
		case span < 2*Day:
			if stationName != "" {
				r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		case (span >= 2*Day) && (span <= 2*Month):
			if stationName != "" {
				r.DB.Table("weather_5m").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_5m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		default:
			if stationName != "" {
				r.DB.Table("weather_1h").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1h").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		}

		log.Infof("returned rows: %v", len(dbFetchedReadings))

		log.Infof("getweatherspan -> spanDuration: %v", span)

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(r.transformSpanReadings(&dbFetchedReadings))
		if err != nil {
			log.Errorf("error marshalling dbFetchedReadings: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}

		w.Write(jsonResponse)
	}
}

func (r *RESTServerStorage) getWeatherLatest(w http.ResponseWriter, req *http.Request) {

	if r.DBEnabled {
		var dbFetchedReadings []BucketReading

		stationName := req.URL.Query().Get("station")

		if stationName != "" {
			r.DB.Table("weather").Limit(1).Where("stationname = ?", stationName).Order("time DESC").Find(&dbFetchedReadings)
		} else {
			r.DB.Table("weather").Limit(1).Order("time DESC").Find(&dbFetchedReadings)
		}

		log.Infof("returned rows: %v", len(dbFetchedReadings))

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(r.transformLatestReadings(&dbFetchedReadings))
		if err != nil {
			log.Errorf("error marshalling dbFetchedReadings: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}

		w.Write(jsonResponse)
	}
}

func (r *RESTServerStorage) transformSpanReadings(dbReadings *[]BucketReading) []*WeatherReading {
	wr := make([]*WeatherReading, 0)

	for _, r := range *dbReadings {
		wr = append(wr, &WeatherReading{
			StationName:           r.StationName,
			ReadingTimestamp:      r.Bucket.UnixMilli(),
			OutsideTemperature:    r.OutTemp,
			ExtraTemp1:            r.ExtraTemp1,
			ExtraTemp2:            r.ExtraTemp2,
			ExtraTemp3:            r.ExtraTemp3,
			ExtraTemp4:            r.ExtraTemp4,
			ExtraTemp5:            r.ExtraTemp5,
			ExtraTemp6:            r.ExtraTemp6,
			ExtraTemp7:            r.ExtraTemp7,
			SoilTemp1:             r.SoilTemp1,
			SoilTemp2:             r.SoilTemp2,
			SoilTemp3:             r.SoilTemp3,
			SoilTemp4:             r.SoilTemp4,
			LeafTemp1:             r.LeafTemp1,
			LeafTemp2:             r.LeafTemp2,
			LeafTemp3:             r.LeafTemp3,
			LeafTemp4:             r.LeafTemp4,
			OutHumidity:           r.OutHumidity,
			ExtraHumidity1:        r.ExtraHumidity1,
			ExtraHumidity2:        r.ExtraHumidity2,
			ExtraHumidity3:        r.ExtraHumidity3,
			ExtraHumidity4:        r.ExtraHumidity4,
			ExtraHumidity5:        r.ExtraHumidity5,
			ExtraHumidity6:        r.ExtraHumidity6,
			ExtraHumidity7:        r.ExtraHumidity7,
			OutsideHumidity:       r.OutHumidity,
			RainRate:              r.RainRate,
			RainIncremental:       r.RainIncremental,
			SolarWatts:            r.SolarWatts,
			SolarJoules:           r.SolarJoules,
			UV:                    r.UV,
			Radiation:             r.Radiation,
			StormRain:             r.StormRain,
			DayRain:               r.DayRain,
			MonthRain:             r.MonthRain,
			YearRain:              r.YearRain,
			Barometer:             r.Barometer,
			WindSpeed:             r.WindSpeed,
			WindDirection:         r.WindDir,
			RainfallDay:           r.DayRain,
			WindChill:             r.WindChill,
			HeatIndex:             r.HeatIndex,
			InsideTemperature:     r.InTemp,
			InsideHumidity:        r.InHumidity,
			ConsBatteryVoltage:    r.ConsBatteryVoltage,
			StationBatteryVoltage: r.StationBatteryVoltage,
		})
	}

	return wr
}

func (r *RESTServerStorage) transformLatestReadings(dbReadings *[]BucketReading) *WeatherReading {
	var latest BucketReading

	if len(*dbReadings) > 0 {
		latest = (*dbReadings)[0]
	} else {
		return &WeatherReading{}
	}
	log.Infof("ts: %v", latest.Timestamp.UnixMilli())
	reading := WeatherReading{
		StationName:           latest.StationName,
		ReadingTimestamp:      latest.Timestamp.UnixMilli(),
		OutsideTemperature:    latest.OutTemp,
		ExtraTemp1:            latest.ExtraTemp1,
		ExtraTemp2:            latest.ExtraTemp2,
		ExtraTemp3:            latest.ExtraTemp3,
		ExtraTemp4:            latest.ExtraTemp4,
		ExtraTemp5:            latest.ExtraTemp5,
		ExtraTemp6:            latest.ExtraTemp6,
		ExtraTemp7:            latest.ExtraTemp7,
		SoilTemp1:             latest.SoilTemp1,
		SoilTemp2:             latest.SoilTemp2,
		SoilTemp3:             latest.SoilTemp3,
		SoilTemp4:             latest.SoilTemp4,
		LeafTemp1:             latest.LeafTemp1,
		LeafTemp2:             latest.LeafTemp2,
		LeafTemp3:             latest.LeafTemp3,
		LeafTemp4:             latest.LeafTemp4,
		OutHumidity:           latest.OutHumidity,
		ExtraHumidity1:        latest.ExtraHumidity1,
		ExtraHumidity2:        latest.ExtraHumidity2,
		ExtraHumidity3:        latest.ExtraHumidity3,
		ExtraHumidity4:        latest.ExtraHumidity4,
		ExtraHumidity5:        latest.ExtraHumidity5,
		ExtraHumidity6:        latest.ExtraHumidity6,
		ExtraHumidity7:        latest.ExtraHumidity7,
		OutsideHumidity:       latest.OutHumidity,
		RainRate:              latest.RainRate,
		RainIncremental:       latest.RainIncremental,
		SolarWatts:            latest.SolarWatts,
		SolarJoules:           latest.SolarJoules,
		UV:                    latest.UV,
		Radiation:             latest.Radiation,
		StormRain:             latest.StormRain,
		DayRain:               latest.DayRain,
		MonthRain:             latest.MonthRain,
		YearRain:              latest.YearRain,
		Barometer:             latest.Barometer,
		WindSpeed:             latest.WindSpeed,
		WindDirection:         latest.WindDir,
		RainfallDay:           latest.DayRain,
		WindChill:             latest.WindChill,
		HeatIndex:             latest.HeatIndex,
		InsideTemperature:     latest.InTemp,
		InsideHumidity:        latest.InHumidity,
		ConsBatteryVoltage:    latest.ConsBatteryVoltage,
		StationBatteryVoltage: latest.StationBatteryVoltage,
	}
	return &reading
}
