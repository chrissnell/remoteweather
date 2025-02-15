package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"net/http"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type WeatherSiteConfig struct {
	StationName      string            `yaml:"station-name,omitempty"`
	PullFromDevice   string            `yaml:"pull-from-device,omitempty"`
	SnowEnabled      bool              `yaml:"snow-enabled,omitempty"`
	SnowDevice       string            `yaml:"snow-device-name,omitempty"`
	SnowBaseDistance float32           `yaml:"snow-base-distance,omitempty"`
	PageTitle        string            `yaml:"page-title,omitempty"`
	AboutStationHTML htmltemplate.HTML `yaml:"about-station-html,omitempty"`
}

// RESTServerConfig describes the YAML-provided configuration for a REST
// server storage backend
type RESTServerConfig struct {
	Cert              string            `yaml:"cert,omitempty"`
	Key               string            `yaml:"key,omitempty"`
	Port              int               `yaml:"port,omitempty"`
	ListenAddr        string            `yaml:"listen-addr,omitempty"`
	WeatherSiteConfig WeatherSiteConfig `yaml:"weather-site,omitempty"`
}

// RESTServerStorage implements a REST server storage backend
type RESTServerStorage struct {
	ClientChans         []chan Reading
	ClientChanMutex     sync.RWMutex
	Server              http.Server
	DB                  *gorm.DB
	DBEnabled           bool
	FS                  *fs.FS
	WeatherSiteConfig   *WeatherSiteConfig
	Devices             []DeviceConfig
	AerisWeatherEnabled bool
}

type SnowReading struct {
	StationName string  `json:"stationname"`
	SnowDepth   float32 `json:"snowdepth"`
	SnowToday   float32 `json:"snowtoday"`
	SnowLast24  float32 `json:"snowlast24"`
	SnowLast72  float32 `json:"snowlast72"`
}

type SnowSeasonReading struct {
	StationName         string  `json:"stationname"`
	TotalSeasonSnowfall float32 `json:"totalseasonsnowfall"`
}

type SnowDeltaResult struct {
	Snowfall float32
}

type WeatherReading struct {
	StationName      string `json:"stationname"`
	StationType      string `json:"stationtype,omitempty"`
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
	SnowDepth             json.Number `json:"snowdepth,omitempty"`
	SnowDistance          json.Number `json:"snowdistance,omitempty"`
	ExtraFloat1           json.Number `json:"extrafloat1,omitempty"`
	ExtraFloat2           json.Number `json:"extrafloat2,omitempty"`
	ExtraFloat3           json.Number `json:"extrafloat3,omitempty"`
	ExtraFloat4           json.Number `json:"extrafloat4,omitempty"`
	ExtraFloat5           json.Number `json:"extrafloat5,omitempty"`
	ExtraFloat6           json.Number `json:"extrafloat6,omitempty"`
	ExtraFloat7           json.Number `json:"extrafloat7,omitempty"`
	ExtraFloat8           json.Number `json:"extrafloat8,omitempty"`
	ExtraFloat9           json.Number `json:"extrafloat9,omitempty"`
	ExtraFloat10          json.Number `json:"extrafloat10,omitempty"`
	ExtraText1            string      `json:"extratext1,omitempty"`
	ExtraText2            string      `json:"extratext2,omitempty"`
	ExtraText3            string      `json:"extratext3,omitempty"`
	ExtraText4            string      `json:"extratext4,omitempty"`
	ExtraText5            string      `json:"extratext5,omitempty"`
	ExtraText6            string      `json:"extratext6,omitempty"`
	ExtraText7            string      `json:"extratext7,omitempty"`
	ExtraText8            string      `json:"extratext8,omitempty"`
	ExtraText9            string      `json:"extratext9,omitempty"`
	ExtraText10           string      `json:"extratext10,omitempty"`
}

const (
	Day   = 24 * time.Hour
	Month = Day * 30
)

var (
	//go:embed all:assets
	content embed.FS
)

// NewRESTServerStorage sets up a new REST server storage backend
func NewRESTServerStorage(ctx context.Context, c *Config) (*RESTServerStorage, error) {
	var err error

	r := new(RESTServerStorage)

	r.Devices = c.Devices

	r.WeatherSiteConfig = &c.Storage.RESTServer.WeatherSiteConfig

	if c.Storage.RESTServer.WeatherSiteConfig.SnowEnabled {
		if !r.snowDeviceExists(c.Storage.RESTServer.WeatherSiteConfig.SnowDevice) {
			log.Fatalln("snow device does not exist:", c.Storage.RESTServer.WeatherSiteConfig.SnowDevice)
		}

		for _, d := range r.Devices {
			if d.Name == c.Storage.RESTServer.WeatherSiteConfig.SnowDevice {
				r.WeatherSiteConfig.SnowBaseDistance = float32(d.BaseSnowDistance)
			}
		}
	}

	// Look to see if the Aeris Weather controller has been configured.
	// If we've configured it, we will enable the /forecast endpoint later on.
	for _, con := range c.Controllers {
		if con.Type == "aerisweather" {
			r.AerisWeatherEnabled = true
		}
	}

	// If a ListenAddr was not provided, listen on all interfaces
	if c.Storage.RESTServer.ListenAddr == "" {
		log.Info("rest.listen_addr not provided; defaulting to 0.0.0.0 (all interfaces)")
		c.Storage.RESTServer.ListenAddr = "0.0.0.0"
	}

	if c.Storage.RESTServer.WeatherSiteConfig.PullFromDevice == "" {
		return &RESTServerStorage{}, fmt.Errorf("pull-from-device must be set")
	} else {
		if !r.validatePullFromStation(c.Storage.RESTServer.WeatherSiteConfig.PullFromDevice) {
			return &RESTServerStorage{}, fmt.Errorf("pull-from-device %v is not a valid station name", c.Storage.RESTServer.WeatherSiteConfig.PullFromDevice)
		}
	}

	fs, _ := fs.Sub(fs.FS(content), "assets")
	r.FS = &fs

	router := mux.NewRouter()
	router.HandleFunc("/span/{span}", r.getWeatherSpan)
	router.HandleFunc("/latest", r.getWeatherLatest)

	if c.Storage.RESTServer.WeatherSiteConfig.SnowEnabled {
		router.HandleFunc("/snow", r.getSnowLatest)
	}

	// We only enable the /forecast endpoint if Aeris Weather has been configured.
	if r.AerisWeatherEnabled {
		router.HandleFunc("/forecast/{span}", r.getForecast)
	}
	router.HandleFunc("/", r.serveIndexTemplate)
	router.HandleFunc("/js/remoteweather.js", r.serveJS)
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

func (r *RESTServerStorage) serveIndexTemplate(w http.ResponseWriter, req *http.Request) {
	view := htmltemplate.Must(htmltemplate.New("index.html.tmpl").ParseFS(*r.FS, "index.html.tmpl"))

	w.Header().Set("Content-Type", "text/html")
	err := view.Execute(w, r.WeatherSiteConfig)
	if err != nil {
		log.Error("error executing template:", err)
		return
	}
}

func (r *RESTServerStorage) serveJS(w http.ResponseWriter, req *http.Request) {
	view := template.Must(template.New("remoteweather.js.tmpl").ParseFS(*r.FS, "remoteweather.js.tmpl"))

	w.Header().Set("Content-Type", "text/javascript")
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
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Info)
		} else {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Warn)
		}

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
		baseDistance := r.WeatherSiteConfig.SnowBaseDistance

		switch {
		case span < 1*Day:
			if stationName != "" {
				r.DB.Table("weather_1m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		case (span >= 1*Day) && (span < 7*Day):
			if stationName != "" {
				r.DB.Table("weather_5m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_5m").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		case (span >= 7*Day) && (span < 2*Month):
			if stationName != "" {
				r.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		default:
			if stationName != "" {
				r.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Where("stationname = ?", stationName).
					Order("bucket").
					Find(&dbFetchedReadings)
			} else {
				r.DB.Table("weather_1h").
					Select("*, (? - snowdistance) AS snowdepth", baseDistance).
					Where("bucket > ?", spanStart).
					Order("bucket").
					Find(&dbFetchedReadings)
			}
		}

		log.Debugf("returned rows: %v", len(dbFetchedReadings))
		log.Debugf("getweatherspan -> spanDuration: %v", span)

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(r.transformSpanReadings(&dbFetchedReadings))
		if err != nil {
			log.Error("error marshaling JSON response:", err)
			http.Error(w, "error: unable to marshal JSON response", 500)
			return
		}

		w.Write(jsonResponse)
	}
}

func (r *RESTServerStorage) getSnowLatest(w http.ResponseWriter, req *http.Request) {

	if r.DBEnabled {
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Info)
		} else {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Warn)
		}

		var dbFetchedReadings []BucketReading

		stationName := req.URL.Query().Get("station")

		if stationName != "" {
			r.DB.Table("weather").Limit(1).Where("stationname = ?", stationName).Order("time DESC").Find(&dbFetchedReadings)
		} else {
			// Client did not supply a station name, so pull from the configured PullFromDevice
			r.DB.Table("weather").Limit(1).Where("stationname = ?", r.WeatherSiteConfig.SnowDevice).Order("time DESC").Find(&dbFetchedReadings)
		}

		log.Debugf("returned rows: %v", len(dbFetchedReadings))

		if len(dbFetchedReadings) > 0 {
			log.Debugf("latest snow reading: %v", mmToInches(dbFetchedReadings[0].SnowDistance))
		}

		var result SnowDeltaResult

		// Get the snowfall since midnight
		query := "SELECT get_new_snow_midnight(?, ?) AS snowfall"
		err := r.DB.Raw(query, r.WeatherSiteConfig.SnowDevice, r.WeatherSiteConfig.SnowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting snow-since-midnight snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow since midnight: %.2f mm\n", result.Snowfall)
		snowSinceMidnight := mmToInches(result.Snowfall)

		// Get the snowfall in the last 24 hours
		query = "SELECT get_new_snow_24h(?, ?) AS snowfall"
		err = r.DB.Raw(query, r.WeatherSiteConfig.SnowDevice, r.WeatherSiteConfig.SnowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 24-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow in last 24h: %.2f mm\n", result.Snowfall)
		snowLast24 := mmToInches(result.Snowfall)

		// Get the snowfall in the last 72 hours
		query = "SELECT get_new_snow_72h(?, ?) AS snowfall"
		err = r.DB.Raw(query, r.WeatherSiteConfig.SnowDevice, r.WeatherSiteConfig.SnowBaseDistance).Scan(&result).Error
		if err != nil {
			log.Errorf("error getting 72-hour snow delta from DB: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}
		log.Debugf("Snow in last 72h: %.2f mm\n", result.Snowfall)
		snowLast72 := mmToInches(result.Snowfall)

		snowReading := SnowReading{
			StationName: r.WeatherSiteConfig.SnowDevice,
			SnowDepth:   mmToInches(r.WeatherSiteConfig.SnowBaseDistance - dbFetchedReadings[0].SnowDistance),
			SnowToday:   float32(snowSinceMidnight),
			SnowLast24:  float32(snowLast24),
			SnowLast72:  float32(snowLast72),
		}

		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		jsonResponse, err := json.Marshal(&snowReading)
		if err != nil {
			log.Errorf("error marshalling snowReading: %v", err)
			http.Error(w, "error fetching readings from DB", 500)
			return
		}

		w.Write(jsonResponse)
	}
}

func (r *RESTServerStorage) getWeatherLatest(w http.ResponseWriter, req *http.Request) {
	if r.DBEnabled {
		// Enable SQL debugging if RW-Debug header is set to "1"
		if req.Header.Get("RW-Debug") == "1" {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Info)
		} else {
			r.DB.Logger = r.DB.Logger.LogMode(logger.Warn)
		}

		var dbFetchedReadings []BucketReading

		stationName := req.URL.Query().Get("station")

		if stationName != "" {
			r.DB.Table("weather").Limit(1).Where("stationname = ?", stationName).Order("time DESC").Find(&dbFetchedReadings)
		} else {
			// Client did not supply a station name, so pull from the configurated PullFromDevice
			r.DB.Table("weather").Limit(1).Where("stationname = ?", r.WeatherSiteConfig.PullFromDevice).Order("time DESC").Find(&dbFetchedReadings)
		}

		type Rainfall struct {
			TotalRain float32
		}

		var todayRainfall Rainfall

		// Fetch the rainfall since midnight
		r.DB.Table("today_rainfall").First(&todayRainfall)

		// Override DayRain from our weather table with the latest data from our view
		dbFetchedReadings[0].DayRain = todayRainfall.TotalRain

		log.Debugf("returned rows: %v", len(dbFetchedReadings))

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

func (r *RESTServerStorage) getForecast(w http.ResponseWriter, req *http.Request) {
	// Enable SQL debugging if RW-Debug header is set to "1"
	if req.Header.Get("RW-Debug") == "1" {
		r.DB.Logger = r.DB.Logger.LogMode(logger.Info)
	} else {
		r.DB.Logger = r.DB.Logger.LogMode(logger.Warn)
	}

	vars := mux.Vars(req)
	span := vars["span"]
	if span == "" {
		log.Errorf("invalid request: missing span duration")
		http.Error(w, "error: missing span duration", 400)
		return
	}

	// 'span' must be between 1 and 4 digits and nothing else
	re := regexp.MustCompile(`^\d{1,4}$`)
	if !re.MatchString(span) {
		log.Errorf("span %v is invalid", span)
		w.WriteHeader(http.StatusBadRequest)
	}

	location := req.URL.Query().Get("location")

	record := AerisWeatherForecastRecord{}

	var result *gorm.DB
	if location != "" {
		result = r.DB.Where("forecast_span_hours = ? AND location = ?", span, location).First(&record)
	} else {
		result = r.DB.Where("forecast_span_hours = ?", span).First(&record)
	}
	if result.RowsAffected == 0 {
		log.Errorf("no forecast records found for span %v", span)
		w.WriteHeader(http.StatusNotFound)
	}

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"lastUpdated\": \"" + record.UpdatedAt.String() + "\", \"data\": "))
	w.Write(record.Data.Bytes)
	w.Write([]byte("}"))
}

func (r *RESTServerStorage) transformSpanReadings(dbReadings *[]BucketReading) []*WeatherReading {
	wr := make([]*WeatherReading, 0)

	for _, r := range *dbReadings {
		wr = append(wr, &WeatherReading{
			StationName:           r.StationName,
			StationType:           r.StationType,
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
			SnowDepth:             float32ToJSONNumber(mmToInches(r.SnowDepth)),
			SnowDistance:          float32ToJSONNumber(r.SnowDistance),
			ExtraFloat1:           float32ToJSONNumber(r.ExtraFloat1),
			ExtraFloat2:           float32ToJSONNumber(r.ExtraFloat2),
			ExtraFloat3:           float32ToJSONNumber(r.ExtraFloat3),
			ExtraFloat4:           float32ToJSONNumber(r.ExtraFloat4),
			ExtraFloat5:           float32ToJSONNumber(r.ExtraFloat5),
			ExtraFloat6:           float32ToJSONNumber(r.ExtraFloat6),
			ExtraFloat7:           float32ToJSONNumber(r.ExtraFloat7),
			ExtraFloat8:           float32ToJSONNumber(r.ExtraFloat8),
			ExtraFloat9:           float32ToJSONNumber(r.ExtraFloat9),
			ExtraFloat10:          float32ToJSONNumber(r.ExtraFloat10),
			ExtraText1:            r.ExtraText1,
			ExtraText2:            r.ExtraText2,
			ExtraText3:            r.ExtraText3,
			ExtraText4:            r.ExtraText4,
			ExtraText5:            r.ExtraText5,
			ExtraText6:            r.ExtraText6,
			ExtraText7:            r.ExtraText7,
			ExtraText8:            r.ExtraText8,
			ExtraText9:            r.ExtraText9,
			ExtraText10:           r.ExtraText10,
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
		StationType:           latest.StationType,
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
		SnowDepth:             float32ToJSONNumber(latest.SnowDepth),
		SnowDistance:          float32ToJSONNumber(latest.SnowDistance),
		ExtraFloat1:           float32ToJSONNumber(latest.ExtraFloat1),
		ExtraFloat2:           float32ToJSONNumber(latest.ExtraFloat2),
		ExtraFloat3:           float32ToJSONNumber(latest.ExtraFloat3),
		ExtraFloat4:           float32ToJSONNumber(latest.ExtraFloat4),
		ExtraFloat5:           float32ToJSONNumber(latest.ExtraFloat5),
		ExtraFloat6:           float32ToJSONNumber(latest.ExtraFloat6),
		ExtraFloat7:           float32ToJSONNumber(latest.ExtraFloat7),
		ExtraFloat8:           float32ToJSONNumber(latest.ExtraFloat8),
		ExtraFloat9:           float32ToJSONNumber(latest.ExtraFloat9),
		ExtraFloat10:          float32ToJSONNumber(latest.ExtraFloat10),
		ExtraText1:            latest.ExtraText1,
		ExtraText2:            latest.ExtraText2,
		ExtraText3:            latest.ExtraText3,
		ExtraText4:            latest.ExtraText4,
		ExtraText5:            latest.ExtraText5,
		ExtraText6:            latest.ExtraText6,
		ExtraText7:            latest.ExtraText7,
		ExtraText8:            latest.ExtraText8,
		ExtraText9:            latest.ExtraText9,
		ExtraText10:           latest.ExtraText10,
	}
	return &reading
}

func (r *RESTServerStorage) validatePullFromStation(pullFromDevice string) bool {
	if len(r.Devices) > 0 {
		for _, station := range r.Devices {
			if station.Name == pullFromDevice {
				return true
			}
		}
	}
	return false
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

func (r *RESTServerStorage) snowDeviceExists(name string) bool {
	for _, device := range r.Devices {
		if device.Name == name && device.Type == "snowgauge" {
			return true
		}
	}
	return false
}
