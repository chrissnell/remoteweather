package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	pb "github.com/chrissnell/remoteweather/protocols/remoteweather"
	serial "github.com/tarm/goserial"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultGRPCTimeout = 5 * time.Second
	maxTries           = 5
	wakeRetries        = 3
)

type forwarderConfig struct {
	// Station connection settings
	serialPort  string
	networkAddr string
	baud        int

	// Destination settings
	grpcServer  string
	stationName string

	// Optional settings
	aprsCallsign string
	latitude     float64
	longitude    float64
	altitude     float64

	// Operational settings
	logLevel string
}

// LoopPacket is the data returned from a Davis Console using the LOOP command
type LoopPacket struct {
	Header             [3]byte
	BarTrend           uint8
	PacketType         uint8
	NextRecord         uint16
	Barometer          uint16
	InTemp             int16
	InHumidity         uint8
	OutTemp            int16
	WindSpeed          uint8
	WindSpeed10        uint8
	WindDir            uint16
	ExtraTemp1         uint8
	ExtraTemp2         uint8
	ExtraTemp3         uint8
	ExtraTemp4         uint8
	ExtraTemp5         uint8
	ExtraTemp6         uint8
	ExtraTemp7         uint8
	SoilTemp1          uint8
	SoilTemp2          uint8
	SoilTemp3          uint8
	SoilTemp4          uint8
	LeafTemp1          uint8
	LeafTemp2          uint8
	LeafTemp3          uint8
	LeafTemp4          uint8
	OutHumidity        uint8
	ExtraHumidity1     uint8
	ExtraHumidity2     uint8
	ExtraHumidity3     uint8
	ExtraHumidity4     uint8
	ExtraHumidity5     uint8
	ExtraHumidity6     uint8
	ExtraHumidity7     uint8
	RainRate           uint16
	UV                 uint8
	Radiation          uint16
	StormRain          uint16
	StormStart         uint16
	DayRain            uint16
	MonthRain          uint16
	YearRain           uint16
	DayET              uint16
	MonthET            uint16
	YearET             uint16
	SoilMoisture1      uint8
	SoilMoisture2      uint8
	SoilMoisture3      uint8
	SoilMoisture4      uint8
	LeafWetness1       uint8
	LeafWetness2       uint8
	LeafWetness3       uint8
	LeafWetness4       uint8
	InsideAlarm        uint8
	RainAlarm          uint8
	OutsideAlarm1      uint8
	OutsideAlarm2      uint8
	ExtraAlarm1        uint8
	ExtraAlarm2        uint8
	ExtraAlarm3        uint8
	ExtraAlarm4        uint8
	ExtraAlarm5        uint8
	ExtraAlarm6        uint8
	ExtraAlarm7        uint8
	ExtraAlarm8        uint8
	SoilLeafAlarm1     uint8
	SoilLeafAlarm2     uint8
	SoilLeafAlarm3     uint8
	SoilLeafAlarm4     uint8
	TxBatteryStatus    uint8
	ConsBatteryVoltage uint16
	ForecastIcon       uint8
	ForecastRule       uint8
	Sunrise            uint16
	Sunset             uint16
	LineFeed           uint8
	CarriageReturn     uint8
	CRC                uint16
}

func main() {
	cfg := parseConfig()

	// Set up logging
	if cfg.logLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("Starting Davis Instruments Forwarder v1.0")
	log.Printf("Station: %s, gRPC Server: %s", cfg.stationName, cfg.grpcServer)

	// Create signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received")
		cancel()
	}()

	// Run the forwarder
	if err := runForwarder(ctx, cfg); err != nil {
		log.Fatalf("Forwarder error: %v", err)
	}

	log.Println("Forwarder shutdown complete")
}

func parseConfig() *forwarderConfig {
	cfg := &forwarderConfig{}

	// Define flags
	flag.StringVar(&cfg.serialPort, "serial", "", "Serial port for Davis station (e.g., /dev/ttyUSB0)")
	flag.StringVar(&cfg.networkAddr, "network", "", "Network address for Davis station (e.g., 192.168.1.100:22222)")
	flag.IntVar(&cfg.baud, "baud", 19200, "Baud rate for serial connection")
	flag.StringVar(&cfg.grpcServer, "server", "", "gRPC server address (e.g., mystation.remoteweather.com:5051) [required]")
	flag.StringVar(&cfg.stationName, "name", "", "Weather station name [required]")
	flag.StringVar(&cfg.aprsCallsign, "aprs", "", "APRS callsign (optional)")
	flag.Float64Var(&cfg.latitude, "lat", 0, "Station latitude (optional)")
	flag.Float64Var(&cfg.longitude, "lon", 0, "Station longitude (optional)")
	flag.Float64Var(&cfg.altitude, "alt", 0, "Station altitude in meters (optional)")
	flag.StringVar(&cfg.logLevel, "log", "info", "Log level (info|debug)")

	flag.Parse()

	// Check environment variables if flags not set
	if cfg.serialPort == "" {
		cfg.serialPort = os.Getenv("DAVIS_SERIAL_PORT")
	}
	if cfg.networkAddr == "" {
		cfg.networkAddr = os.Getenv("DAVIS_NETWORK_ADDR")
	}
	if cfg.grpcServer == "" {
		cfg.grpcServer = os.Getenv("DAVIS_GRPC_SERVER")
	}
	if cfg.stationName == "" {
		cfg.stationName = os.Getenv("DAVIS_STATION_NAME")
	}
	if cfg.aprsCallsign == "" {
		cfg.aprsCallsign = os.Getenv("DAVIS_APRS_CALLSIGN")
	}

	// Validate required configuration
	if cfg.grpcServer == "" {
		fmt.Fprintf(os.Stderr, "Error: gRPC server address is required (--server or DAVIS_GRPC_SERVER)\n")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.stationName == "" {
		fmt.Fprintf(os.Stderr, "Error: Station name is required (--name or DAVIS_STATION_NAME)\n")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.serialPort == "" && cfg.networkAddr == "" {
		fmt.Fprintf(os.Stderr, "Error: Either serial port or network address is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if cfg.serialPort != "" && cfg.networkAddr != "" {
		fmt.Fprintf(os.Stderr, "Error: Cannot specify both serial port and network address\n")
		flag.Usage()
		os.Exit(1)
	}

	return cfg
}

func runForwarder(ctx context.Context, cfg *forwarderConfig) error {
	// Connect to gRPC server
	log.Printf("Connecting to gRPC server at %s", cfg.grpcServer)
	conn, err := grpc.NewClient(cfg.grpcServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	client := pb.NewWeatherV1Client(conn)

	// Set up connection to Davis station
	var rwc io.ReadWriteCloser
	var netConn net.Conn

	if cfg.serialPort != "" {
		log.Printf("Connecting to Davis station via serial port: %s at %d baud", cfg.serialPort, cfg.baud)
		sc := &serial.Config{Name: cfg.serialPort, Baud: cfg.baud}
		rwc, err = serial.OpenPort(sc)
		if err != nil {
			return fmt.Errorf("failed to open serial port: %w", err)
		}
		defer rwc.Close()
	} else {
		log.Printf("Connecting to Davis station via network: %s", cfg.networkAddr)
		netConn, err = net.DialTimeout("tcp", cfg.networkAddr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to network station: %w", err)
		}
		defer netConn.Close()
		rwc = netConn
	}

	// Wake the station
	if err := wakeStation(rwc, cfg); err != nil {
		log.Printf("Warning: Failed to wake station: %v", err)
	}

	// Main reading loop
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Get LOOP packets from Davis station
			if err := getAndForwardLoopPackets(ctx, rwc, netConn, client, cfg); err != nil {
				log.Printf("Error getting/forwarding packets: %v", err)
				// Try to wake the station again
				if err := wakeStation(rwc, cfg); err != nil {
					log.Printf("Warning: Failed to wake station: %v", err)
				}
				// Wait a bit before retrying
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
}

func wakeStation(rwc io.ReadWriteCloser, cfg *forwarderConfig) error {
	for i := 0; i < wakeRetries; i++ {
		// Send newline to wake console
		if _, err := rwc.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to send wake command: %w", err)
		}

		// Read response with timeout
		respChan := make(chan byte, 1)
		go func() {
			buf := make([]byte, 1)
			if _, err := rwc.Read(buf); err == nil {
				respChan <- buf[0]
			}
		}()

		select {
		case resp := <-respChan:
			if resp == '\n' || resp == '\r' {
				if cfg.logLevel == "debug" {
					log.Printf("Station awake (attempt %d)", i+1)
				}
				return nil
			}
		case <-time.After(2 * time.Second):
			if cfg.logLevel == "debug" {
				log.Printf("Wake attempt %d timed out", i+1)
			}
		}
	}

	return fmt.Errorf("failed to wake station after %d attempts", wakeRetries)
}

func getAndForwardLoopPackets(ctx context.Context, rwc io.ReadWriteCloser, netConn net.Conn, client pb.WeatherV1Client, cfg *forwarderConfig) error {
	// Request LOOP packets
	numPackets := 20
	cmd := fmt.Sprintf("LOOP %d\n", numPackets)
	if _, err := rwc.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("failed to send LOOP command: %w", err)
	}

	// Wait a bit for the station to start sending
	time.Sleep(1 * time.Second)

	// Set up scanner for packets
	scanner := bufio.NewScanner(rwc)
	scanner.Split(scanPackets)
	buf := make([]byte, 99)
	scanner.Buffer(buf, 99)

	packetsReceived := 0
	for packetsReceived < numPackets {
		// Set read deadline if network connection
		if netConn != nil {
			if err := netConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return fmt.Errorf("failed to set read deadline: %w", err)
			}
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Scan for next packet
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error scanning packet: %w", err)
			}
			break
		}

		// Parse packet
		data := scanner.Bytes()
		if len(data) != 99 {
			continue
		}

		// Convert to Reading
		reading := convertLoopPacket(data, cfg)

		if cfg.logLevel == "debug" {
			log.Printf("Got reading: Temp=%.1f°F, Humidity=%.0f%%, Pressure=%.2f inHg, Wind=%.0f mph @ %.0f°",
				reading.OutTemp, reading.OutHumidity, reading.Barometer, reading.WindSpeed, reading.WindDir)
		}

		// Forward to gRPC
		if err := forwardReading(ctx, reading, client, cfg); err != nil {
			log.Printf("Failed to forward reading: %v", err)
		} else {
			log.Printf("Successfully forwarded reading to gRPC server")
		}

		packetsReceived++
	}

	return nil
}

// scanPackets is a split function for bufio.Scanner to split Davis LOOP packets
func scanPackets(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for LOOP packet start
	for i := 0; i < len(data)-2; i++ {
		if data[i] == 'L' && data[i+1] == 'O' && data[i+2] == 'O' {
			// Check if we have a full packet (99 bytes)
			if i+99 <= len(data) {
				return i + 99, data[i : i+99], nil
			}
			// Request more data
			return 0, nil, nil
		}
	}

	// No packet found, skip ahead
	if len(data) >= 3 {
		return 1, nil, nil
	}

	// Request more data
	return 0, nil, nil
}

func convertLoopPacket(data []byte, cfg *forwarderConfig) types.Reading {
	r := types.Reading{
		Timestamp:   time.Now(),
		StationName: cfg.stationName,
		StationType: "davis",
	}

	// Parse packet fields (from davis/station.go unpackLoopPacket)
	// Skip header (3 bytes), bar trend (1), packet type (1)

	// Barometer = data[7:9] / 1000.0 (inches Hg)
	if len(data) > 8 {
		r.Barometer = float32(uint16(data[7])|(uint16(data[8])<<8)) / 1000.0
	}

	// Inside temp = data[9:11] / 10.0 (°F)
	if len(data) > 10 {
		r.InTemp = float32(int16(uint16(data[9])|(uint16(data[10])<<8))) / 10.0
	}

	// Inside humidity = data[11]
	if len(data) > 11 {
		r.InHumidity = float32(data[11])
	}

	// Outside temp = data[12:14] / 10.0 (°F)
	if len(data) > 13 {
		r.OutTemp = float32(int16(uint16(data[12])|(uint16(data[13])<<8))) / 10.0
	}

	// Wind speed = data[14] (mph)
	if len(data) > 14 {
		r.WindSpeed = float32(data[14])
	}

	// Wind speed 10 min avg = data[15] (mph)
	if len(data) > 15 {
		r.WindSpeed10 = float32(data[15])
	}

	// Wind direction = data[16:18] (degrees)
	if len(data) > 17 {
		r.WindDir = float32(uint16(data[16]) | (uint16(data[17]) << 8))
	}

	// Outside humidity = data[33]
	if len(data) > 33 {
		r.OutHumidity = float32(data[33])
	}

	// Rain rate = data[41:43] / 100.0 (inches/hr)
	if len(data) > 42 {
		r.RainRate = float32(uint16(data[41])|(uint16(data[42])<<8)) / 100.0
	}

	// UV = data[43]
	if len(data) > 43 {
		r.UV = float32(data[43]) / 10.0
	}

	// Solar radiation = data[44:46] (watts/m²)
	if len(data) > 45 {
		r.SolarWatts = float32(uint16(data[44]) | (uint16(data[45]) << 8))
	}

	// Storm rain = data[46:48] / 100.0 (inches)
	if len(data) > 47 {
		r.StormRain = float32(uint16(data[46])|(uint16(data[47])<<8)) / 100.0
	}

	// Day rain = data[50:52] / 100.0 (inches)
	if len(data) > 51 {
		r.DayRain = float32(uint16(data[50])|(uint16(data[51])<<8)) / 100.0
	}

	// Month rain = data[52:54] / 100.0 (inches)
	if len(data) > 53 {
		r.MonthRain = float32(uint16(data[52])|(uint16(data[53])<<8)) / 100.0
	}

	// Year rain = data[54:56] / 100.0 (inches)
	if len(data) > 55 {
		r.YearRain = float32(uint16(data[54])|(uint16(data[55])<<8)) / 100.0
	}

	// Day ET = data[56:58] / 1000.0 (inches)
	if len(data) > 57 {
		r.DayET = float32(uint16(data[56])|(uint16(data[57])<<8)) / 1000.0
	}

	// Month ET = data[58:60] / 100.0 (inches)
	if len(data) > 59 {
		r.MonthET = float32(uint16(data[58])|(uint16(data[59])<<8)) / 100.0
	}

	// Year ET = data[60:62] / 100.0 (inches)
	if len(data) > 61 {
		r.YearET = float32(uint16(data[60])|(uint16(data[61])<<8)) / 100.0
	}

	// Battery status = data[86]
	if len(data) > 86 {
		r.TxBatteryStatus = uint8(data[86])
	}

	// Console battery voltage = data[87:89] / 300.0 * 512.0 / 100.0
	if len(data) > 88 {
		r.ConsBatteryVoltage = float32(uint16(data[87])|(uint16(data[88])<<8)) * 300.0 / 512.0 / 100.0
	}

	// Forecast icon = data[89]
	if len(data) > 89 {
		r.ForecastIcon = uint8(data[89])
	}

	// Forecast rule = data[90]
	if len(data) > 90 {
		r.ForecastRule = uint8(data[90])
	}

	// TODO: Calculate wind chill and heat index

	return r
}

func forwardReading(ctx context.Context, reading types.Reading, client pb.WeatherV1Client, cfg *forwarderConfig) error {
	// Create gRPC streaming context with timeout
	streamCtx, cancel := context.WithTimeout(ctx, defaultGRPCTimeout)
	defer cancel()

	// Create streaming client
	stream, err := client.SendWeatherReadings(streamCtx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Convert reading to protobuf message
	pbReading := convertToProto(reading, cfg)

	// Send the reading
	if err := stream.Send(pbReading); err != nil {
		return fmt.Errorf("failed to send reading: %w", err)
	}

	// Close the stream and get response
	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("failed to close stream: %w", err)
	}

	return nil
}

func convertToProto(reading types.Reading, cfg *forwarderConfig) *pb.WeatherReading {
	pbReading := &pb.WeatherReading{
		ReadingTimestamp: timestamppb.New(reading.Timestamp),
		StationName:      cfg.stationName,
		StationType:      "davis",

		// Environmental readings
		Barometer:          reading.Barometer,
		InsideTemperature:  reading.InTemp,
		InsideHumidity:     reading.InHumidity,
		OutsideTemperature: reading.OutTemp,
		OutsideHumidity:    reading.OutHumidity,

		// Wind measurements
		WindSpeed:     reading.WindSpeed,
		WindSpeed10:   reading.WindSpeed10,
		WindDirection: reading.WindDir,
		WindChill:     reading.WindChill,

		// Temperature index
		HeatIndex: reading.HeatIndex,

		// Rain measurements
		RainRate:  reading.RainRate,
		DayRain:   reading.DayRain,
		MonthRain: reading.MonthRain,
		YearRain:  reading.YearRain,
		StormRain: reading.StormRain,

		// Solar measurements
		SolarWatts: reading.SolarWatts,
		Uv:         reading.UV,

		// ET measurements
		DayET:   reading.DayET,
		MonthET: reading.MonthET,
		YearET:  reading.YearET,

		// Battery status
		ConsBatteryVoltage: reading.ConsBatteryVoltage,
		TxBatteryStatus:    uint32(reading.TxBatteryStatus),

		// Forecast
		ForecastIcon: uint32(reading.ForecastIcon),
		ForecastRule: uint32(reading.ForecastRule),
	}

	// Add location and APRS info if configured
	if cfg.aprsCallsign != "" {
		pbReading.AprsEnabled = true
		pbReading.AprsCallsign = cfg.aprsCallsign
		pbReading.StationLatitude = cfg.latitude
		pbReading.StationLongitude = cfg.longitude
		pbReading.StationAltitude = cfg.altitude
	}

	return pbReading
}
