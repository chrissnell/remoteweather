package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net"
	"time"
)

// CampbellPacket matches the structure expected by remoteweather
type CampbellPacket struct {
	StationBatteryVoltage float32 `json:"batt_volt,omitempty"`
	OutTemp               float32 `json:"airtemp_f,omitempty"`
	OutHumidity           float32 `json:"rh,omitempty"`
	Barometer             float32 `json:"baro,omitempty"`
	ExtraTemp1            float32 `json:"baro_temp_f,omitempty"`
	SolarWatts            float32 `json:"slr_w,omitempty"`
	SolarJoules           float32 `json:"slr_mj,omitempty"`
	RainIncremental       float32 `json:"rain_in,omitempty"`
	WindSpeed             float32 `json:"wind_s,omitempty"`
	WindDir               uint16  `json:"wind_d,omitempty"`
}

func main() {
	var (
		port     = flag.String("port", "8123", "TCP port to listen on")
		interval = flag.Duration("interval", 2*time.Second, "Interval between readings")
	)
	flag.Parse()

	log.Printf("Campbell Scientific Weather Station Emulator")
	log.Printf("Listening on port %s, sending data every %v", *port, *interval)

	listener, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		log.Printf("Client connected from %s", conn.RemoteAddr())
		go handleConnection(conn, *interval)
	}
}

func handleConnection(conn net.Conn, interval time.Duration) {
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send initial packet immediately
	packet := generateRealisticReading()
	if err := encoder.Encode(packet); err != nil {
		log.Printf("Failed to send packet: %v", err)
		return
	}

	for range ticker.C {
		packet := generateRealisticReading()
		if err := encoder.Encode(packet); err != nil {
			log.Printf("Failed to send packet: %v", err)
			return
		}
		log.Printf("Sent: temp=%.1f°F, humidity=%.1f%%, wind=%.1f@%d°",
			packet.OutTemp, packet.OutHumidity, packet.WindSpeed, packet.WindDir)
	}
}

func generateRealisticReading() CampbellPacket {
	now := time.Now()

	// Generate realistic weather data based on time of day and season
	hour := float64(now.Hour())
	dayOfYear := float64(now.YearDay())

	// Temperature: seasonal variation + daily variation + some randomness
	seasonalTemp := 65.0 + 20.0*math.Sin(2*math.Pi*(dayOfYear-81)/365) // 65°F average, ±20°F seasonal
	dailyTemp := seasonalTemp + 15.0*math.Sin(2*math.Pi*(hour-6)/24)   // ±15°F daily variation, peak at 6PM
	temp := dailyTemp + rand.Float64()*4 - 2                           // ±2°F random variation

	// Humidity: inverse relationship with temperature, higher at night
	baseHumidity := 60 - (temp-65)*0.5 + 15*math.Sin(2*math.Pi*(hour-18)/24)
	humidity := math.Max(10, math.Min(95, baseHumidity+rand.Float64()*10-5))

	// Solar radiation: based on time of day and season
	var solar float32 = 0
	if hour >= 6 && hour <= 18 {
		sunAngle := math.Sin(math.Pi * (hour - 6) / 12)                               // 0 to 1 during daylight
		seasonalSolar := 0.7 + 0.3*math.Sin(2*math.Pi*(dayOfYear-81)/365)             // seasonal variation
		solar = float32(sunAngle * seasonalSolar * 1000 * (0.8 + rand.Float64()*0.4)) // 0-1000W with clouds
	}

	// Wind: more variable
	windSpeed := 3.0 + rand.Float64()*8 + 2*math.Sin(2*math.Pi*hour/24) // 1-13 mph, calmer at night
	windDir := uint16(rand.Intn(360))

	// Rain: occasional light rain
	var rain float32 = 0
	if rand.Float64() < 0.02 { // 2% chance of rain in any reading
		rain = float32(rand.Float64() * 0.01) // 0-0.01 inches
	}

	return CampbellPacket{
		StationBatteryVoltage: 13.2 + float32(rand.Float64()*0.6), // 13.2-13.8V
		OutTemp:               float32(temp),
		OutHumidity:           float32(humidity),
		Barometer:             29.8 + float32(rand.Float64()*0.8), // 29.8-30.6 inHg
		ExtraTemp1:            float32(temp + rand.Float64()*5),   // Slightly higher than air temp
		SolarWatts:            solar,
		SolarJoules:           solar * 0.001, // Convert W to MJ (roughly)
		RainIncremental:       rain,
		WindSpeed:             float32(windSpeed),
		WindDir:               windDir,
	}
}
