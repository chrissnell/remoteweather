package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/pkg/crc16"
)

// FlakyHardwareConfig holds configuration for simulating hardware issues
type FlakyHardwareConfig struct {
	Enabled            bool    // Enable flaky hardware simulation
	DropByteRate       float64 // Probability of dropping random bytes from packets (0.0-1.0)
	CorruptByteRate    float64 // Probability of corrupting random bytes in packets (0.0-1.0)
	DisconnectRate     float64 // Probability of disconnecting during packet transmission (0.0-1.0)
	HangRate           float64 // Probability of hanging/freezing during transmission (0.0-1.0)
	HangDurationMin    int     // Minimum hang duration in seconds
	HangDurationMax    int     // Maximum hang duration in seconds
	BadCRCRate         float64 // Probability of intentionally corrupting CRC (0.0-1.0)
	TruncatePacketRate float64 // Probability of sending truncated packets (0.0-1.0)
	SlowResponseRate   float64 // Probability of very slow responses (0.0-1.0)
	NoResponseRate     float64 // Probability of not responding to commands (0.0-1.0)
}

// LoopPacketWithTrend matches the Davis station implementation
type LoopPacketWithTrend struct {
	Loop               [3]byte // "LOO"
	LoopType           int8    // Always 0
	PacketType         uint8   // 0 = LOOP, 1 = LOOP2
	NextRecord         uint16  // Next archive record number
	Barometer          uint16  // Current barometer reading (inches Hg * 1000)
	InTemp             int16   // Inside temperature (°F * 10)
	InHumidity         uint8   // Inside humidity (%)
	OutTemp            int16   // Outside temperature (°F * 10)
	WindSpeed          uint8   // Wind speed (mph)
	WindSpeed10        uint8   // 10-minute average wind speed (mph)
	WindDir            uint16  // Wind direction (degrees)
	ExtraTemp1         uint8   // Extra temperature 1 (°F + 90)
	ExtraTemp2         uint8   // Extra temperature 2 (°F + 90)
	ExtraTemp3         uint8   // Extra temperature 3 (°F + 90)
	ExtraTemp4         uint8   // Extra temperature 4 (°F + 90)
	ExtraTemp5         uint8   // Extra temperature 5 (°F + 90)
	ExtraTemp6         uint8   // Extra temperature 6 (°F + 90)
	ExtraTemp7         uint8   // Extra temperature 7 (°F + 90)
	SoilTemp1          uint8   // Soil temperature 1 (°F + 90)
	SoilTemp2          uint8   // Soil temperature 2 (°F + 90)
	SoilTemp3          uint8   // Soil temperature 3 (°F + 90)
	SoilTemp4          uint8   // Soil temperature 4 (°F + 90)
	LeafTemp1          uint8   // Leaf temperature 1 (°F + 90)
	LeafTemp2          uint8   // Leaf temperature 2 (°F + 90)
	LeafTemp3          uint8   // Leaf temperature 3 (°F + 90)
	LeafTemp4          uint8   // Leaf temperature 4 (°F + 90)
	OutHumidity        uint8   // Outside humidity (%)
	ExtraHumidity1     uint8   // Extra humidity 1 (%)
	ExtraHumidity2     uint8   // Extra humidity 2 (%)
	ExtraHumidity3     uint8   // Extra humidity 3 (%)
	ExtraHumidity4     uint8   // Extra humidity 4 (%)
	ExtraHumidity5     uint8   // Extra humidity 5 (%)
	ExtraHumidity6     uint8   // Extra humidity 6 (%)
	ExtraHumidity7     uint8   // Extra humidity 7 (%)
	RainRate           uint16  // Rain rate (clicks/hour)
	UV                 uint8   // UV index * 10
	Radiation          uint16  // Solar radiation (watts/m²)
	StormRain          uint16  // Storm rain total (clicks)
	StormStart         uint16  // Storm start date
	DayRain            uint16  // Day rain total (clicks)
	MonthRain          uint16  // Month rain total (clicks)
	YearRain           uint16  // Year rain total (clicks)
	DayET              uint16  // Day evapotranspiration (inches * 1000)
	MonthET            uint16  // Month evapotranspiration (inches * 1000)
	YearET             uint16  // Year evapotranspiration (inches * 1000)
	SoilMoisture1      uint8   // Soil moisture 1 (%)
	SoilMoisture2      uint8   // Soil moisture 2 (%)
	SoilMoisture3      uint8   // Soil moisture 3 (%)
	SoilMoisture4      uint8   // Soil moisture 4 (%)
	LeafWetness1       uint8   // Leaf wetness 1 (0-15)
	LeafWetness2       uint8   // Leaf wetness 2 (0-15)
	LeafWetness3       uint8   // Leaf wetness 3 (0-15)
	LeafWetness4       uint8   // Leaf wetness 4 (0-15)
	InsideAlarm        uint8   // Inside alarm bits
	RainAlarm          uint8   // Rain alarm bits
	OutsideAlarm1      uint8   // Outside alarm 1 bits
	OutsideAlarm2      uint8   // Outside alarm 2 bits
	ExtraAlarm1        uint8   // Extra alarm 1 bits
	ExtraAlarm2        uint8   // Extra alarm 2 bits
	ExtraAlarm3        uint8   // Extra alarm 3 bits
	ExtraAlarm4        uint8   // Extra alarm 4 bits
	ExtraAlarm5        uint8   // Extra alarm 5 bits
	ExtraAlarm6        uint8   // Extra alarm 6 bits
	ExtraAlarm7        uint8   // Extra alarm 7 bits
	ExtraAlarm8        uint8   // Extra alarm 8 bits
	SoilLeafAlarm1     uint8   // Soil/leaf alarm 1 bits
	SoilLeafAlarm2     uint8   // Soil/leaf alarm 2 bits
	SoilLeafAlarm3     uint8   // Soil/leaf alarm 3 bits
	SoilLeafAlarm4     uint8   // Soil/leaf alarm 4 bits
	TxBatteryStatus    uint8   // Transmitter battery status
	ConsBatteryVoltage uint16  // Console battery voltage ((volts * 300) + 0.5)
	ForecastIcon       uint8   // Forecast icon
	ForecastRule       uint8   // Forecast rule number
	Sunrise            uint16  // Sunrise time (BCD HHMM)
	Sunset             uint16  // Sunset time (BCD HHMM)
	Trend              int8    // 3-hour barometer trend
	_                  [5]byte // Padding to make 99 bytes
	CRC                uint16  // CRC16 checksum
}

type WeatherEmulator struct {
	baseTemp     float64
	baseHumidity float64
	basePressure float64
	startTime    time.Time
	flakyConfig  FlakyHardwareConfig
}

func NewWeatherEmulator(flakyConfig FlakyHardwareConfig) *WeatherEmulator {
	return &WeatherEmulator{
		baseTemp:     70.0, // Base temperature in °F
		baseHumidity: 50.0, // Base humidity in %
		basePressure: 30.0, // Base pressure in inches Hg
		startTime:    time.Now(),
		flakyConfig:  flakyConfig,
	}
}

func (w *WeatherEmulator) GenerateLoopPacket() *LoopPacketWithTrend {
	now := time.Now()
	hourOfDay := float64(now.Hour()) + float64(now.Minute())/60.0
	dayOfYear := float64(now.YearDay())

	// Create seasonal and daily temperature variation
	seasonalTemp := 20.0 * math.Sin(2*math.Pi*(dayOfYear-80)/365.0) // ±20°F seasonal variation
	dailyTemp := 15.0 * math.Sin(2*math.Pi*(hourOfDay-6)/24.0)      // ±15°F daily variation
	tempNoise := (rand.Float64() - 0.5) * 4.0                       // ±2°F random noise

	currentTemp := w.baseTemp + seasonalTemp + dailyTemp + tempNoise

	// Humidity inversely related to temperature
	currentHumidity := w.baseHumidity + (w.baseTemp-currentTemp)*0.8 + (rand.Float64()-0.5)*10.0
	if currentHumidity < 10 {
		currentHumidity = 10
	}
	if currentHumidity > 95 {
		currentHumidity = 95
	}

	// Pressure with slight random walk
	pressureChange := (rand.Float64() - 0.5) * 0.02
	w.basePressure += pressureChange
	if w.basePressure < 28.5 {
		w.basePressure = 28.5
	}
	if w.basePressure > 31.5 {
		w.basePressure = 31.5
	}

	// Wind with gusts
	baseWind := 5.0 + rand.Float64()*10.0 // 5-15 mph base
	windGust := rand.Float64() * 8.0      // Up to 8 mph gust
	windSpeed := baseWind + windGust
	windDir := uint16(rand.Float64() * 360)

	// Solar radiation based on time of day
	var solarRad uint16
	if hourOfDay > 6 && hourOfDay < 18 {
		solarFactor := math.Sin(math.Pi * (hourOfDay - 6) / 12.0)
		solarRad = uint16(1000 * solarFactor * (0.7 + rand.Float64()*0.3))
	}

	packet := &LoopPacketWithTrend{
		Loop:               [3]byte{'L', 'O', 'O'},
		LoopType:           0,
		PacketType:         0,
		NextRecord:         uint16(rand.Intn(2048)),
		Barometer:          uint16(w.basePressure * 1000),
		InTemp:             int16((currentTemp + 2) * 10), // Inside slightly warmer
		InHumidity:         uint8(currentHumidity - 5),    // Inside slightly drier
		OutTemp:            int16(currentTemp * 10),
		WindSpeed:          uint8(windSpeed),
		WindSpeed10:        uint8(baseWind),
		WindDir:            windDir,
		ExtraTemp1:         255, // Not connected
		ExtraTemp2:         255,
		ExtraTemp3:         255,
		ExtraTemp4:         255,
		ExtraTemp5:         255,
		ExtraTemp6:         255,
		ExtraTemp7:         255,
		SoilTemp1:          uint8(currentTemp + 90 - 5), // Soil cooler
		SoilTemp2:          255,
		SoilTemp3:          255,
		SoilTemp4:          255,
		LeafTemp1:          uint8(currentTemp + 90),
		LeafTemp2:          255,
		LeafTemp3:          255,
		LeafTemp4:          255,
		OutHumidity:        uint8(currentHumidity),
		ExtraHumidity1:     255,
		ExtraHumidity2:     255,
		ExtraHumidity3:     255,
		ExtraHumidity4:     255,
		ExtraHumidity5:     255,
		ExtraHumidity6:     255,
		ExtraHumidity7:     255,
		RainRate:           uint16(rand.Intn(3)), // Occasional light rain
		UV:                 uint8(float64(solarRad) / 100),
		Radiation:          solarRad,
		StormRain:          uint16(rand.Intn(50)),
		StormStart:         0,
		DayRain:            uint16(rand.Intn(20)),
		MonthRain:          uint16(rand.Intn(200)),
		YearRain:           uint16(rand.Intn(1000)),
		DayET:              uint16(rand.Intn(300)),
		MonthET:            uint16(rand.Intn(3000)),
		YearET:             uint16(rand.Intn(36000)),
		SoilMoisture1:      uint8(30 + rand.Intn(40)), // 30-70%
		SoilMoisture2:      255,
		SoilMoisture3:      255,
		SoilMoisture4:      255,
		LeafWetness1:       uint8(rand.Intn(16)),
		LeafWetness2:       255,
		LeafWetness3:       255,
		LeafWetness4:       255,
		InsideAlarm:        0,
		RainAlarm:          0,
		OutsideAlarm1:      0,
		OutsideAlarm2:      0,
		ExtraAlarm1:        0,
		ExtraAlarm2:        0,
		ExtraAlarm3:        0,
		ExtraAlarm4:        0,
		ExtraAlarm5:        0,
		ExtraAlarm6:        0,
		ExtraAlarm7:        0,
		ExtraAlarm8:        0,
		SoilLeafAlarm1:     0,
		SoilLeafAlarm2:     0,
		SoilLeafAlarm3:     0,
		SoilLeafAlarm4:     0,
		TxBatteryStatus:    0,
		ConsBatteryVoltage: 4050, // 13.5V * 300 + 0.5
		ForecastIcon:       2,    // Partly cloudy
		ForecastRule:       45,
		Sunrise:            0x0630,                           // 6:30 AM BCD
		Sunset:             0x1945,                           // 7:45 PM BCD
		Trend:              int8((rand.Float64() - 0.5) * 6), // ±3 trend
	}

	return packet
}

func (packet *LoopPacketWithTrend) ToBytes() []byte {
	// Convert struct to byte slice without CRC
	data := make([]byte, 97) // 99 bytes total - 2 bytes CRC

	copy(data[0:3], packet.Loop[:])
	data[3] = byte(packet.LoopType)
	data[4] = packet.PacketType
	binary.LittleEndian.PutUint16(data[5:7], packet.NextRecord)
	binary.LittleEndian.PutUint16(data[7:9], packet.Barometer)
	binary.LittleEndian.PutUint16(data[9:11], uint16(packet.InTemp))
	data[11] = packet.InHumidity
	binary.LittleEndian.PutUint16(data[12:14], uint16(packet.OutTemp))
	data[14] = packet.WindSpeed
	data[15] = packet.WindSpeed10
	binary.LittleEndian.PutUint16(data[16:18], packet.WindDir)

	// Extra temperatures
	data[18] = packet.ExtraTemp1
	data[19] = packet.ExtraTemp2
	data[20] = packet.ExtraTemp3
	data[21] = packet.ExtraTemp4
	data[22] = packet.ExtraTemp5
	data[23] = packet.ExtraTemp6
	data[24] = packet.ExtraTemp7

	// Soil temperatures
	data[25] = packet.SoilTemp1
	data[26] = packet.SoilTemp2
	data[27] = packet.SoilTemp3
	data[28] = packet.SoilTemp4

	// Leaf temperatures
	data[29] = packet.LeafTemp1
	data[30] = packet.LeafTemp2
	data[31] = packet.LeafTemp3
	data[32] = packet.LeafTemp4

	// Humidity values
	data[33] = packet.OutHumidity
	data[34] = packet.ExtraHumidity1
	data[35] = packet.ExtraHumidity2
	data[36] = packet.ExtraHumidity3
	data[37] = packet.ExtraHumidity4
	data[38] = packet.ExtraHumidity5
	data[39] = packet.ExtraHumidity6
	data[40] = packet.ExtraHumidity7

	// Rain and weather data
	binary.LittleEndian.PutUint16(data[41:43], packet.RainRate)
	data[43] = packet.UV
	binary.LittleEndian.PutUint16(data[44:46], packet.Radiation)
	binary.LittleEndian.PutUint16(data[46:48], packet.StormRain)
	binary.LittleEndian.PutUint16(data[48:50], packet.StormStart)
	binary.LittleEndian.PutUint16(data[50:52], packet.DayRain)
	binary.LittleEndian.PutUint16(data[52:54], packet.MonthRain)
	binary.LittleEndian.PutUint16(data[54:56], packet.YearRain)
	binary.LittleEndian.PutUint16(data[56:58], packet.DayET)
	binary.LittleEndian.PutUint16(data[58:60], packet.MonthET)
	binary.LittleEndian.PutUint16(data[60:62], packet.YearET)

	// Soil moisture
	data[62] = packet.SoilMoisture1
	data[63] = packet.SoilMoisture2
	data[64] = packet.SoilMoisture3
	data[65] = packet.SoilMoisture4

	// Leaf wetness
	data[66] = packet.LeafWetness1
	data[67] = packet.LeafWetness2
	data[68] = packet.LeafWetness3
	data[69] = packet.LeafWetness4

	// Alarms
	data[70] = packet.InsideAlarm
	data[71] = packet.RainAlarm
	data[72] = packet.OutsideAlarm1
	data[73] = packet.OutsideAlarm2
	data[74] = packet.ExtraAlarm1
	data[75] = packet.ExtraAlarm2
	data[76] = packet.ExtraAlarm3
	data[77] = packet.ExtraAlarm4
	data[78] = packet.ExtraAlarm5
	data[79] = packet.ExtraAlarm6
	data[80] = packet.ExtraAlarm7
	data[81] = packet.ExtraAlarm8
	data[82] = packet.SoilLeafAlarm1
	data[83] = packet.SoilLeafAlarm2
	data[84] = packet.SoilLeafAlarm3
	data[85] = packet.SoilLeafAlarm4

	// Battery and forecast
	data[86] = packet.TxBatteryStatus
	binary.LittleEndian.PutUint16(data[87:89], packet.ConsBatteryVoltage)
	data[89] = packet.ForecastIcon
	data[90] = packet.ForecastRule
	binary.LittleEndian.PutUint16(data[91:93], packet.Sunrise)
	binary.LittleEndian.PutUint16(data[93:95], packet.Sunset)
	data[95] = byte(packet.Trend)
	data[96] = 0 // Padding

	// Calculate CRC16 for the first 97 bytes only
	crc := crc16.Crc16(data[:97])
	result := make([]byte, 99)
	copy(result, data)
	// Store CRC in bytes 97-98 (big-endian as per Davis protocol)
	binary.BigEndian.PutUint16(result[97:99], crc)

	return result
}

// simulateHardwareIssues applies various hardware problems to packet data
func (w *WeatherEmulator) simulateHardwareIssues(packet []byte) []byte {
	if !w.flakyConfig.Enabled {
		return packet
	}

	result := make([]byte, len(packet))
	copy(result, packet)

	// Simulate dropped bytes
	if rand.Float64() < w.flakyConfig.DropByteRate {
		dropCount := 1 + rand.Intn(3) // Drop 1-3 bytes
		for i := 0; i < dropCount && len(result) > 3; i++ {
			dropPos := 3 + rand.Intn(len(result)-3) // Don't drop LOO header
			result = append(result[:dropPos], result[dropPos+1:]...)
			log.Printf("FLAKY: Dropped byte at position %d", dropPos)
		}
	}

	// Simulate corrupted bytes
	if rand.Float64() < w.flakyConfig.CorruptByteRate {
		corruptCount := 1 + rand.Intn(2) // Corrupt 1-2 bytes
		for i := 0; i < corruptCount && len(result) > 3; i++ {
			corruptPos := 3 + rand.Intn(len(result)-3) // Don't corrupt LOO header
			originalByte := result[corruptPos]
			result[corruptPos] = byte(rand.Intn(256))
			log.Printf("FLAKY: Corrupted byte at position %d: 0x%02X -> 0x%02X",
				corruptPos, originalByte, result[corruptPos])
		}
	}

	// Simulate truncated packets
	if rand.Float64() < w.flakyConfig.TruncatePacketRate {
		truncateAt := 10 + rand.Intn(len(result)-10) // Keep at least 10 bytes
		result = result[:truncateAt]
		log.Printf("FLAKY: Truncated packet to %d bytes (was %d)", len(result), len(packet))
	}

	// Simulate bad CRC (corrupt the CRC bytes specifically)
	if rand.Float64() < w.flakyConfig.BadCRCRate && len(result) >= 99 {
		result[97] = byte(rand.Intn(256))
		result[98] = byte(rand.Intn(256))
		log.Printf("FLAKY: Corrupted CRC bytes")
	}

	return result
}

// shouldHang determines if the emulator should hang/freeze
func (w *WeatherEmulator) shouldHang() bool {
	return w.flakyConfig.Enabled && rand.Float64() < w.flakyConfig.HangRate
}

// shouldDisconnect determines if the emulator should disconnect
func (w *WeatherEmulator) shouldDisconnect() bool {
	return w.flakyConfig.Enabled && rand.Float64() < w.flakyConfig.DisconnectRate
}

// shouldRespondSlowly determines if the emulator should respond very slowly
func (w *WeatherEmulator) shouldRespondSlowly() bool {
	return w.flakyConfig.Enabled && rand.Float64() < w.flakyConfig.SlowResponseRate
}

// shouldNotRespond determines if the emulator should ignore commands
func (w *WeatherEmulator) shouldNotRespond() bool {
	return w.flakyConfig.Enabled && rand.Float64() < w.flakyConfig.NoResponseRate
}

// hangForRandomDuration simulates the device hanging/freezing
func (w *WeatherEmulator) hangForRandomDuration() {
	if !w.flakyConfig.Enabled {
		return
	}

	duration := w.flakyConfig.HangDurationMin +
		rand.Intn(w.flakyConfig.HangDurationMax-w.flakyConfig.HangDurationMin+1)
	log.Printf("FLAKY: Hanging for %d seconds...", duration)
	time.Sleep(time.Duration(duration) * time.Second)
	log.Printf("FLAKY: Resuming after hang")
}

func handleConnection(conn net.Conn, emulator *WeatherEmulator) {
	defer conn.Close()

	log.Printf("New Davis station connection from %s", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		command := scanner.Text()
		log.Printf("Received command: %q", command)

		// Check if we should ignore this command (simulate unresponsive hardware)
		if emulator.shouldNotRespond() {
			log.Printf("FLAKY: Ignoring command (no response)")
			continue
		}

		// Check if we should respond very slowly
		if emulator.shouldRespondSlowly() {
			slowDelay := 5 + rand.Intn(10) // 5-15 second delay
			log.Printf("FLAKY: Responding slowly (waiting %d seconds)", slowDelay)
			time.Sleep(time.Duration(slowDelay) * time.Second)
		}

		// Check if we should hang before processing
		if emulator.shouldHang() {
			emulator.hangForRandomDuration()
		}

		switch {
		case command == "" || command == "\n" || command == "\r":
			// Wake command - respond with line feed and carriage return

			// Check for random disconnection during wake response
			if emulator.shouldDisconnect() {
				log.Printf("FLAKY: Disconnecting during wake response")
				return
			}

			conn.Write([]byte("\n\r"))
			log.Printf("Sent wake response")

		case command == "LPS 2 1":
			// LOOP command (older format) - send ACK then 20 LOOP packets
			conn.Write([]byte("\x06")) // ACK
			log.Printf("Sent ACK for LPS LOOP command")

			// Send 20 LOOP packets
			for i := 0; i < 20; i++ {
				// Check for disconnection before sending packet
				if emulator.shouldDisconnect() {
					log.Printf("FLAKY: Disconnecting during LPS packet %d/20 transmission", i+1)
					return
				}

				// Check for hang before generating packet
				if emulator.shouldHang() {
					emulator.hangForRandomDuration()
				}

				packet := emulator.GenerateLoopPacket()
				packetBytes := packet.ToBytes()

				// Apply hardware issues to the packet
				flakyPacketBytes := emulator.simulateHardwareIssues(packetBytes)

				n, err := conn.Write(flakyPacketBytes)
				if err != nil {
					log.Printf("Error sending LOOP packet %d: %v", i+1, err)
					return
				}

				if len(flakyPacketBytes) != len(packetBytes) {
					log.Printf("Sent FLAKY LPS packet %d/20 (%d bytes, original %d): temp=%.1f°F, humidity=%d%%, pressure=%.2f\"",
						i+1, n, len(packetBytes), float64(packet.OutTemp)/10.0, packet.OutHumidity, float64(packet.Barometer)/1000.0)
				} else {
					log.Printf("Sent LPS packet %d/20 (%d bytes): temp=%.1f°F, humidity=%d%%, pressure=%.2f\"",
						i+1, n, float64(packet.OutTemp)/10.0, packet.OutHumidity, float64(packet.Barometer)/1000.0)
				}

				time.Sleep(1500 * time.Millisecond) // 1.5 second delay between packets
			}

		case strings.HasPrefix(command, "LOOP "):
			// Standard LOOP command format: "LOOP n"
			parts := strings.Fields(command)
			if len(parts) != 2 {
				log.Printf("Invalid LOOP command format: %q", command)
				conn.Write([]byte("\x15")) // RESEND (NACK)
				continue
			}

			numPackets, err := strconv.Atoi(parts[1])
			if err != nil || numPackets <= 0 || numPackets > 2048 {
				log.Printf("Invalid LOOP packet count: %q", parts[1])
				conn.Write([]byte("\x15")) // RESEND (NACK)
				continue
			}

			// Send ACK first
			conn.Write([]byte("\x06")) // ACK
			log.Printf("Sent ACK for LOOP %d command", numPackets)

			// Send the requested number of LOOP packets
			for i := 0; i < numPackets; i++ {
				// Check for disconnection before sending packet
				if emulator.shouldDisconnect() {
					log.Printf("FLAKY: Disconnecting during packet %d/%d transmission", i+1, numPackets)
					return
				}

				// Check for hang before generating packet
				if emulator.shouldHang() {
					emulator.hangForRandomDuration()
				}

				packet := emulator.GenerateLoopPacket()
				packetBytes := packet.ToBytes()

				// Apply hardware issues to the packet
				flakyPacketBytes := emulator.simulateHardwareIssues(packetBytes)

				n, err := conn.Write(flakyPacketBytes)
				if err != nil {
					log.Printf("Error sending LOOP packet %d: %v", i+1, err)
					return
				}

				if len(flakyPacketBytes) != len(packetBytes) {
					log.Printf("Sent FLAKY LOOP packet %d/%d (%d bytes, original %d): temp=%.1f°F, humidity=%d%%, pressure=%.2f\"",
						i+1, numPackets, n, len(packetBytes), float64(packet.OutTemp)/10.0, packet.OutHumidity, float64(packet.Barometer)/1000.0)
				} else {
					log.Printf("Sent LOOP packet %d/%d (%d bytes): temp=%.1f°F, humidity=%d%%, pressure=%.2f\"",
						i+1, numPackets, n, float64(packet.OutTemp)/10.0, packet.OutHumidity, float64(packet.Barometer)/1000.0)
				}

				time.Sleep(1500 * time.Millisecond) // 1.5 second delay between packets
			}

		default:
			log.Printf("Unknown command: %q", command)
			conn.Write([]byte("\x15")) // RESEND (NACK)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Connection error: %v", err)
	}

	log.Printf("Davis station connection from %s closed", conn.RemoteAddr())
}

func main() {
	var (
		port = flag.Int("port", 22222, "Port to listen on")

		// Flaky hardware simulation flags
		flaky              = flag.Bool("flaky", false, "Enable flaky hardware simulation")
		dropByteRate       = flag.Float64("drop-rate", 0.05, "Probability of dropping bytes from packets (0.0-1.0)")
		corruptByteRate    = flag.Float64("corrupt-rate", 0.05, "Probability of corrupting bytes in packets (0.0-1.0)")
		disconnectRate     = flag.Float64("disconnect-rate", 0.02, "Probability of disconnecting during transmission (0.0-1.0)")
		hangRate           = flag.Float64("hang-rate", 0.01, "Probability of hanging/freezing (0.0-1.0)")
		hangDurationMin    = flag.Int("hang-min", 3, "Minimum hang duration in seconds")
		hangDurationMax    = flag.Int("hang-max", 8, "Maximum hang duration in seconds")
		badCRCRate         = flag.Float64("bad-crc-rate", 0.03, "Probability of corrupting CRC (0.0-1.0)")
		truncatePacketRate = flag.Float64("truncate-rate", 0.02, "Probability of truncating packets (0.0-1.0)")
		slowResponseRate   = flag.Float64("slow-rate", 0.02, "Probability of very slow responses (0.0-1.0)")
		noResponseRate     = flag.Float64("no-response-rate", 0.01, "Probability of not responding to commands (0.0-1.0)")
	)
	flag.Parse()

	log.Printf("Starting Davis Weather Station Emulator on port %d", *port)
	if *flaky {
		log.Printf("FLAKY HARDWARE MODE ENABLED:")
		log.Printf("  Drop bytes: %.1f%%, Corrupt bytes: %.1f%%, Bad CRC: %.1f%%",
			*dropByteRate*100, *corruptByteRate*100, *badCRCRate*100)
		log.Printf("  Truncate: %.1f%%, Disconnect: %.1f%%, Hang: %.1f%%",
			*truncatePacketRate*100, *disconnectRate*100, *hangRate*100)
		log.Printf("  Slow response: %.1f%%, No response: %.1f%%",
			*slowResponseRate*100, *noResponseRate*100)
		log.Printf("  Hang duration: %d-%d seconds", *hangDurationMin, *hangDurationMax)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	flakyConfig := FlakyHardwareConfig{
		Enabled:            *flaky,
		DropByteRate:       *dropByteRate,
		CorruptByteRate:    *corruptByteRate,
		DisconnectRate:     *disconnectRate,
		HangRate:           *hangRate,
		HangDurationMin:    *hangDurationMin,
		HangDurationMax:    *hangDurationMax,
		BadCRCRate:         *badCRCRate,
		TruncatePacketRate: *truncatePacketRate,
		SlowResponseRate:   *slowResponseRate,
		NoResponseRate:     *noResponseRate,
	}
	emulator := NewWeatherEmulator(flakyConfig)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping server...")
		cancel()
		listener.Close()
	}()

	log.Printf("Davis emulator listening on port %d", *port)
	log.Println("Connect RemoteWeather with: hostname: localhost, port:", *port)

	for {
		select {
		case <-ctx.Done():
			log.Println("Server stopped")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return // Server is shutting down
				}
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			go handleConnection(conn, emulator)
		}
	}
}
