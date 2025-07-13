package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
	pb "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"github.com/panjf2000/gnet/v2"
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

// davisNetworkClient handles the gnet-based network connection for Davis stations
type davisNetworkClient struct {
	*gnet.BuiltinEventEngine
	
	addr           string
	conn           gnet.Conn
	readChan       chan []byte
	writeChan      chan []byte
	errorChan      chan error
	connectedChan  chan bool
	closeChan      chan struct{}
	mu             sync.Mutex
	buffer         *bytes.Buffer
	packetScanner  *bufio.Scanner
	cfg            *forwarderConfig
	grpcClient     pb.WeatherV1Client
	ctx            context.Context
}

func (c *davisNetworkClient) OnBoot(eng gnet.Engine) (action gnet.Action) {
	log.Printf("gnet engine started for Davis network client")
	return gnet.None
}

func (c *davisNetworkClient) OnOpen(conn gnet.Conn) (out []byte, action gnet.Action) {
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	
	log.Printf("Connected to Davis station at %s", c.addr)
	c.connectedChan <- true
	
	// Send initial wake command
	return []byte("\n"), gnet.None
}

func (c *davisNetworkClient) OnClose(conn gnet.Conn, err error) (action gnet.Action) {
	log.Printf("Connection closed: %v", err)
	c.mu.Lock()
	c.conn = nil
	c.mu.Unlock()
	
	if err != nil {
		select {
		case c.errorChan <- err:
		default:
		}
	}
	
	return gnet.Shutdown
}

func (c *davisNetworkClient) OnTraffic(conn gnet.Conn) (action gnet.Action) {
	data, err := conn.Next(-1)
	if err != nil {
		log.Printf("Error reading data: %v", err)
		return gnet.Close
	}
	
	if len(data) == 0 {
		return gnet.None
	}
	
	// Handle wake response
	if len(data) == 1 && (data[0] == '\n' || data[0] == '\r') {
		if c.cfg.logLevel == "debug" {
			log.Printf("Received wake response from station")
		}
		return gnet.None
	}
	
	// Add data to buffer
	c.mu.Lock()
	c.buffer.Write(data)
	c.mu.Unlock()
	
	// Process any complete packets
	c.processPackets()
	
	return gnet.None
}

func (c *davisNetworkClient) processPackets() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Look for complete LOOP packets in the buffer
	data := c.buffer.Bytes()
	
	for i := 0; i < len(data)-2; i++ {
		if data[i] == 'L' && data[i+1] == 'O' && data[i+2] == 'O' {
			// Check if we have a full packet (99 bytes)
			if i+99 <= len(data) {
				packet := data[i : i+99]
				
				// Process the packet
				reading := convertLoopPacket(packet, c.cfg)
				
				if c.cfg.logLevel == "debug" {
					log.Printf("Got reading: Temp=%.1f°F, Humidity=%.0f%%, Pressure=%.2f inHg, Wind=%.0f mph @ %.0f°",
						reading.OutTemp, reading.OutHumidity, reading.Barometer, reading.WindSpeed, reading.WindDir)
				}
				
				// Forward to gRPC
				go func() {
					if err := forwardReading(c.ctx, reading, c.grpcClient, c.cfg); err != nil {
						log.Printf("Failed to forward reading: %v", err)
					} else {
						log.Printf("Successfully forwarded reading to gRPC server")
					}
				}()
				
				// Remove processed data from buffer
				c.buffer = bytes.NewBuffer(data[i+99:])
				return
			}
		}
	}
	
	// Remove old data if buffer is getting too large
	if c.buffer.Len() > 1000 {
		c.buffer = bytes.NewBuffer(data[len(data)-100:])
	}
}

func (c *davisNetworkClient) Write(data []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	
	if conn == nil {
		return fmt.Errorf("not connected")
	}
	
	return conn.AsyncWrite(data, nil)
}

func main() {
	cfg := parseConfig()

	// Set up logging
	if cfg.logLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("Starting Davis Instruments Forwarder (gnet) v1.0")
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
	flag.StringVar(&cfg.grpcServer, "server", "", "gRPC server address (e.g., localhost:50051) [required]")
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

	if cfg.serialPort != "" {
		return runSerialForwarder(ctx, cfg, client)
	} else {
		return runNetworkForwarder(ctx, cfg, client)
	}
}

func runSerialForwarder(ctx context.Context, cfg *forwarderConfig, client pb.WeatherV1Client) error {
	log.Printf("Connecting to Davis station via serial port: %s at %d baud", cfg.serialPort, cfg.baud)
	
	sc := &serial.Config{Name: cfg.serialPort, Baud: cfg.baud}
	rwc, err := serial.OpenPort(sc)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}
	defer rwc.Close()

	// Wake the station
	if err := wakeSerialStation(rwc, cfg); err != nil {
		log.Printf("Warning: Failed to wake station: %v", err)
	}

	// Main reading loop
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Get LOOP packets from Davis station
			if err := getAndForwardSerialPackets(ctx, rwc, client, cfg); err != nil {
				log.Printf("Error getting/forwarding packets: %v", err)
				// Try to wake the station again
				if err := wakeSerialStation(rwc, cfg); err != nil {
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

func runNetworkForwarder(ctx context.Context, cfg *forwarderConfig, grpcClient pb.WeatherV1Client) error {
	log.Printf("Starting gnet-based network forwarder for %s", cfg.networkAddr)
	
	// Create the network client
	client := &davisNetworkClient{
		addr:          cfg.networkAddr,
		readChan:      make(chan []byte, 100),
		writeChan:     make(chan []byte, 100),
		errorChan:     make(chan error, 10),
		connectedChan: make(chan bool, 1),
		closeChan:     make(chan struct{}),
		buffer:        bytes.NewBuffer(nil),
		cfg:           cfg,
		grpcClient:    grpcClient,
		ctx:           ctx,
	}
	
	// Start gnet client
	clientDone := make(chan error, 1)
	go func() {
		err := gnet.Run(client, "tcp://"+cfg.networkAddr, 
			gnet.WithMulticore(false),
			gnet.WithReusePort(false),
			gnet.WithTicker(false),
		)
		clientDone <- err
	}()
	
	// Wait for connection
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-client.connectedChan:
		log.Println("Connected to Davis station")
	case err := <-clientDone:
		return fmt.Errorf("gnet client failed to start: %w", err)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout connecting to Davis station")
	}
	
	// Request LOOP packets periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	// Request initial LOOP packets
	if err := client.Write([]byte("LOOP 20\n")); err != nil {
		log.Printf("Failed to send initial LOOP command: %v", err)
	}
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := client.Write([]byte("LOOP 20\n")); err != nil {
				log.Printf("Failed to send LOOP command: %v", err)
			}
		case err := <-client.errorChan:
			log.Printf("Network error: %v", err)
			return err
		case err := <-clientDone:
			return fmt.Errorf("gnet client stopped: %w", err)
		}
	}
}

func wakeSerialStation(rwc io.ReadWriteCloser, cfg *forwarderConfig) error {
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

func getAndForwardSerialPackets(ctx context.Context, rwc io.ReadWriteCloser, client pb.WeatherV1Client, cfg *forwarderConfig) error {
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
		ReadingTimestamp:     timestamppb.New(reading.Timestamp),
		StationName:         cfg.stationName,
		StationType:         "davis",

		// Environmental readings
		Barometer:           reading.Barometer,
		InsideTemperature:   reading.InTemp,
		InsideHumidity:      reading.InHumidity,
		OutsideTemperature:  reading.OutTemp,
		OutsideHumidity:     reading.OutHumidity,

		// Wind measurements
		WindSpeed:           reading.WindSpeed,
		WindSpeed10:         reading.WindSpeed10,
		WindDirection:       reading.WindDir,
		WindChill:           reading.WindChill,

		// Temperature index
		HeatIndex:           reading.HeatIndex,

		// Rain measurements
		RainRate:            reading.RainRate,
		DayRain:             reading.DayRain,
		MonthRain:           reading.MonthRain,
		YearRain:            reading.YearRain,
		StormRain:           reading.StormRain,

		// Solar measurements
		SolarWatts:          reading.SolarWatts,
		Uv:                  reading.UV,

		// ET measurements
		DayET:               reading.DayET,
		MonthET:             reading.MonthET,
		YearET:              reading.YearET,

		// Battery status
		ConsBatteryVoltage:  reading.ConsBatteryVoltage,
		TxBatteryStatus:     uint32(reading.TxBatteryStatus),

		// Forecast
		ForecastIcon:        uint32(reading.ForecastIcon),
		ForecastRule:        uint32(reading.ForecastRule),
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