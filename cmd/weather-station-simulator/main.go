// Package main provides a multi-protocol weather station simulator supporting Davis and Campbell formats.
package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/pkg/crc16"
)

// -------------------------------- Davis Structures --------------------------------

// LoopPacketWithTrend mirrors structure used in real Davis stations (size 99 bytes + CRC)
// Only subset fields needed for RemoteWeather; others kept for size.
//
//go:generate no
//nolint:revive // large struct with exported fields to match protocol
type LoopPacketWithTrend struct {
	Loop        [3]byte
	LoopType    int8
	PacketType  uint8
	NextRecord  uint16
	Barometer   uint16
	InTemp      int16
	InHumidity  uint8
	OutTemp     int16
	WindSpeed   uint8
	WindSpeed10 uint8
	WindDir     uint16
	_           [70]byte // pad to 93 so total incl. CRC is 99+2
	Trend       int8
}

// WeatherEmulator generates synthetic weather data.
type WeatherEmulator struct {
	baseTemp     float64
	baseHumidity float64
	basePressure float64
	startTime    time.Time
}

func NewWeatherEmulator() *WeatherEmulator {
	return &WeatherEmulator{
		baseTemp:     70,
		baseHumidity: 55,
		basePressure: 30,
		startTime:    time.Now(),
	}
}

func (w *WeatherEmulator) GenerateLoopPacket() []byte {
	now := time.Now()
	hour := float64(now.Hour()) + float64(now.Minute())/60
	day := float64(now.YearDay())

	seasonal := 20 * math.Sin(2*math.Pi*(day-81)/365)
	daily := 15 * math.Sin(2*math.Pi*(hour-6)/24)

	temp := w.baseTemp + seasonal + daily + (rand.Float64()-0.5)*4
	humidity := math.Max(10, math.Min(95, w.baseHumidity+(w.baseTemp-temp)))
	pressure := w.basePressure + (rand.Float64()-0.5)*0.05
	wind := 5 + rand.Float64()*10
	windDir := uint16(rand.Float64() * 360)

	pkt := &LoopPacketWithTrend{}
	copy(pkt.Loop[:], []byte{'L', 'O', 'O'})
	pkt.Barometer = uint16(pressure * 1000)
	pkt.InTemp = int16((temp + 2) * 10)
	pkt.InHumidity = uint8(humidity - 3)
	pkt.OutTemp = int16(temp * 10)
	pkt.WindSpeed = uint8(wind)
	pkt.WindSpeed10 = uint8(wind)
	pkt.WindDir = windDir
	pkt.Trend = 0

	return pkt.ToBytes()
}

func (p *LoopPacketWithTrend) ToBytesWithoutCRC() []byte {
	// Build the 97-byte Davis LOOP body (without CRC)
	data := make([]byte, 97)
	writer := data[:0]
	writer = append(writer, p.Loop[:]...)
	writer = append(writer, byte(p.LoopType))
	writer = append(writer, p.PacketType)
	writer = binary.LittleEndian.AppendUint16(writer, p.NextRecord)
	writer = binary.LittleEndian.AppendUint16(writer, p.Barometer)
	writer = binary.LittleEndian.AppendUint16(writer, uint16(p.InTemp))
	writer = append(writer, p.InHumidity)
	writer = binary.LittleEndian.AppendUint16(writer, uint16(p.OutTemp))
	writer = append(writer, p.WindSpeed)
	writer = append(writer, p.WindSpeed10)
	writer = binary.LittleEndian.AppendUint16(writer, p.WindDir)
	// pad remaining bytes with 0xFF until we have 96 bytes, leaving 1 for Trend
	for len(writer) < 96 {
		writer = append(writer, 0xFF)
	}
	writer = append(writer, byte(p.Trend))
	copy(data, writer)
	return data
}

func (p *LoopPacketWithTrend) ToBytes() []byte {
	body := p.ToBytesWithoutCRC()
	crc := crc16.Crc16(body)
	packet := make([]byte, 0, 99)
	packet = append(packet, body...)
	packet = binary.BigEndian.AppendUint16(packet, crc)
	return packet
}

// -------------------------------- Campbell Structures --------------------------------

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

func generateCampbellReading() CampbellPacket {
	now := time.Now()
	hour := float64(now.Hour())
	day := float64(now.YearDay())

	seasonalTemp := 65 + 20*math.Sin(2*math.Pi*(day-81)/365)
	dailyTemp := seasonalTemp + 15*math.Sin(2*math.Pi*(hour-6)/24)
	temp := dailyTemp + rand.Float64()*4 - 2

	humidity := math.Max(10, math.Min(95, 60-(temp-65)*0.5+(rand.Float64()*10-5)))

	solar := float32(0)
	if hour >= 6 && hour <= 18 {
		sunAngle := math.Sin(math.Pi * (hour - 6) / 12)
		solar = float32(sunAngle * 1000 * (0.8 + rand.Float64()*0.4))
	}

	wind := 3 + rand.Float64()*8 + 2*math.Sin(2*math.Pi*hour/24)

	rain := float32(0)
	if rand.Float64() < 0.02 {
		rain = float32(rand.Float64() * 0.01)
	}

	return CampbellPacket{
		StationBatteryVoltage: 13.2 + float32(rand.Float64()*0.6),
		OutTemp:               float32(temp),
		OutHumidity:           float32(humidity),
		Barometer:             29.8 + float32(rand.Float64()*0.8),
		ExtraTemp1:            float32(temp + rand.Float64()*5),
		SolarWatts:            solar,
		SolarJoules:           solar * 0.001,
		RainIncremental:       rain,
		WindSpeed:             float32(wind),
		WindDir:               uint16(rand.Intn(360)),
	}
}

// -------------------------------- Main --------------------------------

func main() {
	var intervalSec = flag.Int("interval", 2, "seconds between readings")
	flag.Parse()

	log.Printf("Weather-Station Simulator starting: Davis TCP 22222, Campbell TCP 7000, interval %ds", *intervalSec)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	// Shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Start Davis listener
	wg.Add(1)
	go startDavisListener(ctx, wg, ":22222", time.Duration(*intervalSec)*time.Second)

	// Start Campbell listener
	wg.Add(1)
	go startCampbellListener(ctx, wg, ":7100", time.Duration(*intervalSec)*time.Second)

	wg.Wait()
}

// ---------------------------- Davis server ----------------------------

func startDavisListener(ctx context.Context, wg *sync.WaitGroup, addr string, interval time.Duration) {
	defer wg.Done()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Davis] failed to listen on %s: %v", addr, err)
		return
	}
	defer l.Close()
	log.Printf("[Davis] listening on %s", addr)

	emulator := NewWeatherEmulator()

	for {
		connChan := make(chan net.Conn)
		go func() {
			c, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
				default:
					log.Printf("[Davis] accept error: %v", err)
				}
				return
			}
			connChan <- c
		}()

		select {
		case <-ctx.Done():
			return
		case conn := <-connChan:
			log.Printf("[Davis] client %s connected", conn.RemoteAddr())
			go handleDavisClient(ctx, conn, emulator, interval)
		}
	}
}

func handleDavisClient(ctx context.Context, conn net.Conn, emu *WeatherEmulator, _ time.Duration) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	wakeDone := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Short read deadline so we can exit when context is done.
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		line, err := reader.ReadString('\n')
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue // nothing new, keep waiting
			}
			return // connection error or closed by peer
		}

		cmd := strings.TrimSpace(strings.ToUpper(line))

		// Wake-up handshake: any blank/\n/\r line gets a "\n\r" reply once.
		if !wakeDone {
			conn.Write([]byte("\n\r"))
			wakeDone = true
			continue
		}

		// Handle LOOP n command exactly like legacy emulator.
		if strings.HasPrefix(cmd, "LOOP") {
			parts := strings.Fields(cmd)
			if len(parts) != 2 {
				// Invalid format – send NACK and continue.
				conn.Write([]byte{0x15})
				continue
			}

			count, perr := strconv.Atoi(parts[1])
			if perr != nil || count <= 0 || count > 2048 {
				conn.Write([]byte{0x15})
				continue
			}

			// ACK.
			conn.Write([]byte{0x06})

			for i := 0; i < count; i++ {
				pkt := emu.GenerateLoopPacket()
				if _, werr := conn.Write(pkt); werr != nil {
					log.Printf("[Davis] write error: %v", werr)
					return
				}
				// Davis consoles transmit every ~1.5 s.
				select {
				case <-ctx.Done():
					return
				case <-time.After(1500 * time.Millisecond):
				}
			}
		}
	}
}

// -------------------------- Campbell server ---------------------------

func startCampbellListener(ctx context.Context, wg *sync.WaitGroup, addr string, interval time.Duration) {
	defer wg.Done()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Campbell] failed to listen on %s: %v", addr, err)
		return
	}
	defer l.Close()
	log.Printf("[Campbell] listening on %s", addr)

	for {
		connChan := make(chan net.Conn)
		go func() {
			c, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
				default:
					log.Printf("[Campbell] accept error: %v", err)
				}
				return
			}
			connChan <- c
		}()

		select {
		case <-ctx.Done():
			return
		case conn := <-connChan:
			log.Printf("[Campbell] client %s connected", conn.RemoteAddr())
			go handleCampbellClient(ctx, conn, interval)
		}
	}
}

func handleCampbellClient(ctx context.Context, conn net.Conn, interval time.Duration) {
	defer conn.Close()
	encoder := json.NewEncoder(conn)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pkt := generateCampbellReading()
			if err := encoder.Encode(pkt); err != nil {
				log.Printf("[Campbell] write error: %v", err)
				return
			}
		}
	}
}
