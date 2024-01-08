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
	StationName      string        `yaml:"station_name,omitempty"`
	PageTitle        string        `yaml:"page_title,omitempty"`
	AboutStationHTML template.HTML `yaml:"about_station_html,omitempty"`
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
	OutsideTemperature    json.Number `json:"otemp,omitempty"`
	ExtraTemp1            json.Number `json:"extratemp1,omitempty"`
	ExtraTemp2            json.Number `json:"extratemp2,omitempty"`
	ExtraTemp3            json.Number `json:"extratemp3,omitempty"`
	ExtraTemp4            json.Number `json:"extratemp4,omitempty"`
	ExtraTemp5            json.Number `json:"extratemp5,omitempty"`
	ExtraTemp6            json.Number `json:"extratemp6,omitempty"`
	ExtraTemp7            json.Number `json:"extratemp7,omitempty"`
	SoilTemp1             json.Number `json:"soiltemp1,omitempty"`
	SoilTemp2             json.Number `json:"soiltemp2,omitempty"`
	SoilTemp3             json.Number `json:"soiltemp3,omitempty"`
	SoilTemp4             json.Number `json:"soiltemp4,omitempty"`
	LeafTemp1             json.Number `json:"leaftemp1,omitempty"`
	LeafTemp2             json.Number `json:"leaftemp2,omitempty"`
	LeafTemp3             json.Number `json:"leaftemp3,omitempty"`
	LeafTemp4             json.Number `json:"leaftemp4,omitempty"`
	OutHumidity           json.Number `json:"outhumidity,omitempty"`
	ExtraHumidity1        json.Number `json:"extrahumidity1,omitempty"`
	ExtraHumidity2        json.Number `json:"extrahumidity2,omitempty"`
	ExtraHumidity3        json.Number `json:"extrahumidity3,omitempty"`
	ExtraHumidity4        json.Number `json:"extrahumidity4,omitempty"`
	ExtraHumidity5        json.Number `json:"extrahumidity5,omitempty"`
	ExtraHumidity6        json.Number `json:"extrahumidity6,omitempty"`
	ExtraHumidity7        json.Number `json:"extrahumidity7,omitempty"`
	OutsideHumidity       json.Number `json:"ohum,omitempty"`
	RainRate              json.Number `json:"rainrate,omitempty"`
	RainIncremental       json.Number `json:"rainincremental,omitempty"`
	SolarWatts            json.Number `json:"solarwatts,omitempty"`
	SolarJoules           json.Number `json:"solarjoules,omitempty"`
	UV                    json.Number `json:"uv,omitempty"`
	Radiation             json.Number `json:"radiation,omitempty"`
	StormRain             json.Number `json:"stormrain,omitempty"`
	DayRain               json.Number `json:"dayrain,omitempty"`
	MonthRain             json.Number `json:"monthrain,omitempty"`
	YearRain              json.Number `json:"yearrain,omitempty"`
	Barometer             json.Number `json:"bar,omitempty"`
	WindSpeed             json.Number `json:"winds,omitempty"`
	WindDirection         json.Number `json:"windd,omitempty"`
	CardinalDirection     string      `json:"windcard,omitempty"`
	RainfallDay           json.Number `json:"rainday,omitempty"`
	WindChill             json.Number `json:"windch,omitempty"`
	HeatIndex             json.Number `json:"heatidx,omitempty"`
	InsideTemperature     json.Number `json:"itemp,omitempty"`
	InsideHumidity        json.Number `json:"ihum,omitempty"`
	ConsBatteryVoltage    json.Number `json:"consbatteryvoltage,omitempty"`
	StationBatteryVoltage json.Number `json:"stationbatteryvoltage,omitempty"`
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
	router.PathPrefix("/").Handler(http.FileServer(http.FS(*r.FS)))

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

func (r *RESTServerStorage) serveAboutTemplate(w http.ResponseWriter, req *http.Request) {
	view := template.Must(template.New("about.html.tmpl").ParseFS(*r.FS, "about.html.tmpl"))

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

		switch {
		case span < 1*Day:
			if stationName != "" {
				r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		case (span >= 1*Day) && (span < 7*Day):
			if stationName != "" {
				r.DB.Table("weather_5m").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_5m").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		case (span >= 7*Day) && (span < 2*Month):
			if stationName != "" {
				r.DB.Table("weather_1h").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1h").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
			}
		default:
			if stationName != "" {
				r.DB.Table("weather_1d").Where("bucket > ?", spanStart).Where("stationname = ?", stationName).Order("bucket").Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1d").Where("bucket > ?", spanStart).Order("bucket").Find(&dbFetchedReadings)
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
			OutsideTemperature:    float32ToJSONNumber(r.OutTemp),
			ExtraTemp1:            float32ToJSONNumber(r.ExtraTemp1),
			ExtraTemp2:            float32ToJSONNumber(r.ExtraTemp2),
			ExtraTemp3:            float32ToJSONNumber(r.ExtraTemp3),
			ExtraTemp4:            float32ToJSONNumber(r.ExtraTemp4),
			ExtraTemp5:            float32ToJSONNumber(r.ExtraTemp5),
			ExtraTemp6:            float32ToJSONNumber(r.ExtraTemp6),
			ExtraTemp7:            float32ToJSONNumber(r.ExtraTemp7),
			SoilTemp1:             float32ToJSONNumber(r.SoilTemp1),
			SoilTemp2:             float32ToJSONNumber(r.SoilTemp2),
			SoilTemp3:             float32ToJSONNumber(r.SoilTemp3),
			SoilTemp4:             float32ToJSONNumber(r.SoilTemp4),
			LeafTemp1:             float32ToJSONNumber(r.LeafTemp1),
			LeafTemp2:             float32ToJSONNumber(r.LeafTemp2),
			LeafTemp3:             float32ToJSONNumber(r.LeafTemp3),
			LeafTemp4:             float32ToJSONNumber(r.LeafTemp4),
			OutHumidity:           float32ToJSONNumber(r.OutHumidity),
			ExtraHumidity1:        float32ToJSONNumber(r.ExtraHumidity1),
			ExtraHumidity2:        float32ToJSONNumber(r.ExtraHumidity2),
			ExtraHumidity3:        float32ToJSONNumber(r.ExtraHumidity3),
			ExtraHumidity4:        float32ToJSONNumber(r.ExtraHumidity4),
			ExtraHumidity5:        float32ToJSONNumber(r.ExtraHumidity5),
			ExtraHumidity6:        float32ToJSONNumber(r.ExtraHumidity6),
			ExtraHumidity7:        float32ToJSONNumber(r.ExtraHumidity7),
			OutsideHumidity:       float32ToJSONNumber(r.OutHumidity),
			RainRate:              float32ToJSONNumber(r.RainRate),
			RainIncremental:       float32ToJSONNumber(r.RainIncremental),
			SolarWatts:            float32ToJSONNumber(r.SolarWatts),
			SolarJoules:           float32ToJSONNumber(r.SolarJoules),
			UV:                    float32ToJSONNumber(r.UV),
			Radiation:             float32ToJSONNumber(r.Radiation),
			StormRain:             float32ToJSONNumber(r.StormRain),
			DayRain:               float32ToJSONNumber(r.DayRain),
			MonthRain:             float32ToJSONNumber(r.MonthRain),
			YearRain:              float32ToJSONNumber(r.YearRain),
			Barometer:             float32ToJSONNumber(r.Barometer),
			WindSpeed:             float32ToJSONNumber(r.WindSpeed),
			WindDirection:         float32ToJSONNumber(r.WindDir),
			CardinalDirection:     headingToCardinalDirection(r.WindDir),
			RainfallDay:           float32ToJSONNumber(r.DayRain),
			WindChill:             float32ToJSONNumber(r.WindChill),
			HeatIndex:             float32ToJSONNumber(r.HeatIndex),
			InsideTemperature:     float32ToJSONNumber(r.InTemp),
			InsideHumidity:        float32ToJSONNumber(r.InHumidity),
			ConsBatteryVoltage:    float32ToJSONNumber(r.ConsBatteryVoltage),
			StationBatteryVoltage: float32ToJSONNumber(r.StationBatteryVoltage),
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
	reading := WeatherReading{
		StationName:           latest.StationName,
		ReadingTimestamp:      latest.Timestamp.UnixMilli(),
		OutsideTemperature:    float32ToJSONNumber(latest.OutTemp),
		ExtraTemp1:            float32ToJSONNumber(latest.ExtraTemp1),
		ExtraTemp2:            float32ToJSONNumber(latest.ExtraTemp2),
		ExtraTemp3:            float32ToJSONNumber(latest.ExtraTemp3),
		ExtraTemp4:            float32ToJSONNumber(latest.ExtraTemp4),
		ExtraTemp5:            float32ToJSONNumber(latest.ExtraTemp5),
		ExtraTemp6:            float32ToJSONNumber(latest.ExtraTemp6),
		ExtraTemp7:            float32ToJSONNumber(latest.ExtraTemp7),
		SoilTemp1:             float32ToJSONNumber(latest.SoilTemp1),
		SoilTemp2:             float32ToJSONNumber(latest.SoilTemp2),
		SoilTemp3:             float32ToJSONNumber(latest.SoilTemp3),
		SoilTemp4:             float32ToJSONNumber(latest.SoilTemp4),
		LeafTemp1:             float32ToJSONNumber(latest.LeafTemp1),
		LeafTemp2:             float32ToJSONNumber(latest.LeafTemp2),
		LeafTemp3:             float32ToJSONNumber(latest.LeafTemp3),
		LeafTemp4:             float32ToJSONNumber(latest.LeafTemp4),
		OutHumidity:           float32ToJSONNumber(latest.OutHumidity),
		ExtraHumidity1:        float32ToJSONNumber(latest.ExtraHumidity1),
		ExtraHumidity2:        float32ToJSONNumber(latest.ExtraHumidity2),
		ExtraHumidity3:        float32ToJSONNumber(latest.ExtraHumidity3),
		ExtraHumidity4:        float32ToJSONNumber(latest.ExtraHumidity4),
		ExtraHumidity5:        float32ToJSONNumber(latest.ExtraHumidity5),
		ExtraHumidity6:        float32ToJSONNumber(latest.ExtraHumidity6),
		ExtraHumidity7:        float32ToJSONNumber(latest.ExtraHumidity7),
		OutsideHumidity:       float32ToJSONNumber(latest.OutHumidity),
		RainRate:              float32ToJSONNumber(latest.RainRate),
		RainIncremental:       float32ToJSONNumber(latest.RainIncremental),
		SolarWatts:            float32ToJSONNumber(latest.SolarWatts),
		SolarJoules:           float32ToJSONNumber(latest.SolarJoules),
		UV:                    float32ToJSONNumber(latest.UV),
		Radiation:             float32ToJSONNumber(latest.Radiation),
		StormRain:             float32ToJSONNumber(latest.StormRain),
		DayRain:               float32ToJSONNumber(latest.DayRain),
		MonthRain:             float32ToJSONNumber(latest.MonthRain),
		YearRain:              float32ToJSONNumber(latest.YearRain),
		Barometer:             float32ToJSONNumber(latest.Barometer),
		WindSpeed:             float32ToJSONNumber(latest.WindSpeed),
		WindDirection:         float32ToJSONNumber(latest.WindDir),
		CardinalDirection:     headingToCardinalDirection(latest.WindDir),
		RainfallDay:           float32ToJSONNumber(latest.DayRain),
		WindChill:             float32ToJSONNumber(latest.WindChill),
		HeatIndex:             float32ToJSONNumber(latest.HeatIndex),
		InsideTemperature:     float32ToJSONNumber(latest.InTemp),
		InsideHumidity:        float32ToJSONNumber(latest.InHumidity),
		ConsBatteryVoltage:    float32ToJSONNumber(latest.ConsBatteryVoltage),
		StationBatteryVoltage: float32ToJSONNumber(latest.StationBatteryVoltage),
	}
	return &reading
}

func float32ToJSONNumber(f float32) json.Number {
	var s string
	if f == float32(int32(f)) {
		s = fmt.Sprintf("%.1f", f) // 1 decimal if integer
	} else {
		s = fmt.Sprint(f)
	}
	return json.Number(s)
}

func headingToCardinalDirection(f float32) string {
	cardDirections := []string{"N", "NNE", "NE", "ENE",
		"E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW",
		"W", "WNW", "NW", "NNW"}

	cardIndex := int((float32(f) + float32(11.25)) / float32(22.5))
	return cardDirections[cardIndex%16]
}
