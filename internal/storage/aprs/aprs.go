package aprs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/internal/types"
	aprspkg "github.com/chrissnell/remoteweather/pkg/aprs"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// CurrentReading is a Reading + a mutex that maintains the most recent reading from
// the station for whenever we need to send one to APRS-IS
type CurrentReading struct {
	r types.Reading
	sync.RWMutex
}

// Storage holds general configuration related to our APRS/CWOP transmissions
type Storage struct {
	ctx             context.Context
	configProvider  config.ConfigProvider
	APRSReadingChan chan types.Reading
	currentReading  *CurrentReading
}

// New sets up a new APRS-IS storage backend
func New(configProvider config.ConfigProvider) (Storage, error) {
	a := Storage{}

	// Load configuration to validate APRS is configured
	storageConfig, err := configProvider.GetStorageConfig()
	if err != nil {
		return a, fmt.Errorf("error loading storage configuration: %v", err)
	}

	if storageConfig.APRS == nil || storageConfig.APRS.Server == "" {
		return a, fmt.Errorf("APRS storage configuration is missing")
	}

	// Check if we have at least one station APRS config
	stationConfigs, err := configProvider.GetStationAPRSConfigs()
	if err != nil {
		return a, fmt.Errorf("error loading station APRS configurations: %v", err)
	}

	if len(stationConfigs) == 0 {
		return a, fmt.Errorf("you must configure at least one station APRS configuration")
	}

	// Validate at least one station has a callsign and location
	validStation := false
	for _, station := range stationConfigs {
		if station.Callsign != "" && station.Location.Lat != 0 && station.Location.Lon != 0 {
			validStation = true
			break
		}
	}

	if !validStation {
		return a, fmt.Errorf("you must provide a callsign and location for at least one station in the APRS configuration")
	}

	a.configProvider = configProvider
	a.APRSReadingChan = make(chan types.Reading, 10)

	return a, nil
}

// StartStorageEngine creates a goroutine loop to receive readings and send
// them off to APRS-IS when needed
func (a Storage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting APRS-IS storage engine...")
	a.ctx = ctx
	readingChan := make(chan types.Reading)

	a.currentReading = &CurrentReading{}
	a.currentReading.r = types.Reading{}
	go a.processMetrics(ctx, wg, readingChan)
	go a.sendReports(ctx, wg)
	return readingChan
}

func (a *Storage) sendReports(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	// Kick off our first report manually
	goodReading := 0
	for goodReading == 0 {
		a.currentReading.RLock()
		if a.currentReading.r.Timestamp.Unix() > 0 {
			go a.sendReadingToAPRSIS(ctx, wg)
			goodReading++
		}
		a.currentReading.RUnlock()
		time.Sleep(1 * time.Second)
	}

	for {
		select {
		case <-ticker.C:
			a.currentReading.RLock()
			if a.currentReading.r.Timestamp.Unix() > 0 {
				go a.sendReadingToAPRSIS(ctx, wg)
			}
			a.currentReading.RUnlock()

		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling sendReports()")
			return
		}
	}

}

func (a *Storage) sendReadingToAPRSIS(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	connectionTimeout := 3 * time.Second

	pkt := a.CreateCompleteWeatherReport('/', '_')
	log.Debugf("sending reading to APRS-IS: %+v", pkt)

	// Load APRS storage configuration
	storageConfig, err := a.configProvider.GetStorageConfig()
	if err != nil {
		log.Error("error loading storage configuration for APRS-IS: %v", err)
		return
	}

	if storageConfig.APRS == nil || storageConfig.APRS.Server == "" {
		log.Error("APRS storage configuration is missing or incomplete")
		return
	}

	// Get station APRS configurations
	stationConfigs, err := a.configProvider.GetStationAPRSConfigs()
	if err != nil {
		log.Error("error loading station APRS configurations: %v", err)
		return
	}

	var stationConfig *config.StationAPRSData
	for _, station := range stationConfigs {
		if station.Enabled && station.Callsign != "" {
			stationConfig = &station
			break
		}
	}

	if stationConfig == nil {
		log.Error("no enabled station APRS configuration found")
		return
	}

	dialer := net.Dialer{
		Timeout: connectionTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", storageConfig.APRS.Server)
	if err != nil {
		log.Error("error dialing APRS-IS server %v: %v",
			storageConfig.APRS.Server, err)
		return
	}
	defer conn.Close()

	buffCon := bufio.NewReader(conn)

	conn.SetReadDeadline(time.Now().Add(connectionTimeout))

	resp, err := buffCon.ReadString('\n')
	if err != nil {
		log.Error("error writing to APRS-IS server:", err)
		return
	}

	if resp[0] != '#' {
		log.Error("APRS-IS server did not respond with proper greeting:", string(resp))
		return
	}

	// Calculate passcode from callsign
	passcode := aprspkg.CalculatePasscode(stationConfig.Callsign)

	login := fmt.Sprintf("user %v pass %v vers remoteweather-%v\r\n",
		stationConfig.Callsign, passcode, constants.Version)

	conn.Write([]byte(login))

	conn.SetReadDeadline(time.Now().Add(connectionTimeout))

	resp, err = buffCon.ReadString('\n')
	if err != nil {
		log.Error("error writing to APRS-IS server:", err)
		return
	}

	if resp[0] != '#' {
		log.Error("error: APRS-IS server did not respond with proper login reply:", string(resp))
		return
	}

	if !strings.Contains(string(resp), "verified") {
		log.Error("error: unable to log into APRS-IS.  Server response:", string(resp))
		return
	}

	conn.Write([]byte(pkt + "\r\n"))
}

func (a *Storage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan types.Reading) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case r := <-rchan:
			err := a.StoreCurrentReading(r)
			if err != nil {
				log.Error(err)
			}
		case <-ctx.Done():
			log.Info("cancellation request recieved.  Cancelling processMetrics().")
			return
		}
	}
}

// StoreCurrentReading stores the latest reading in our object
func (a *Storage) StoreCurrentReading(r types.Reading) error {
	a.currentReading.Lock()
	a.currentReading.r = r
	a.currentReading.Unlock()
	return nil
}

// CreateCompleteWeatherReport creates an APRS weather report with compressed position
// report included.
func (a *Storage) CreateCompleteWeatherReport(symTable, symCode rune) string {
	var buffer bytes.Buffer

	// Get station APRS configurations
	stationConfigs, err := a.configProvider.GetStationAPRSConfigs()
	if err != nil {
		log.Error("error loading station APRS configurations for weather report: %v", err)
		return ""
	}

	var stationConfig *config.StationAPRSData
	for _, station := range stationConfigs {
		if station.Enabled && station.Callsign != "" {
			stationConfig = &station
			break
		}
	}

	if stationConfig == nil {
		log.Error("no enabled station APRS configuration found for weather report")
		return ""
	}

	a.currentReading.RLock()
	defer a.currentReading.RUnlock()

	// Build callsign and position
	callsign := strings.ToUpper(stationConfig.Callsign)
	lat := stationConfig.Location.Lat
	lon := stationConfig.Location.Lon

	latAPRS := convertLatitudeToAPRSFormat(lat)
	lonAPRS := convertLongitudeToAPRSFormat(lon)

	// Our callsign comes first.
	buffer.WriteString(callsign)

	// Then we add our APRS path
	buffer.WriteString(">APRS,TCPIP:")

	// Next byte in our compressed weather report is the data type indicator.
	// The rune '!' indicates a real-time compressed position report
	buffer.WriteRune('!')

	// Next, we write our latitude
	buffer.WriteString(latAPRS)

	// Next byte is the symbol table selector
	buffer.WriteRune(symTable)

	// Then we write our longitude
	buffer.WriteString(lonAPRS)

	// Then our symbol code
	buffer.WriteRune(symCode)

	// Then our wind direction and speed
	buffer.WriteString(fmt.Sprintf("%03d/%03d", int(a.currentReading.r.WindDir), int(a.currentReading.r.WindSpeed)))

	// We don't keep track of gusts
	buffer.WriteString("g...")

	// Then we add our temperature reading
	buffer.WriteString(fmt.Sprintf("t%03d", int64(a.currentReading.r.OutTemp)))

	// Then we add our rainfall since midnight
	buffer.WriteString(fmt.Sprintf("P%03d", int64(a.currentReading.r.DayRain*100)))

	// Then we add our humidity
	buffer.WriteString(fmt.Sprintf("h%02d", int64(a.currentReading.r.OutHumidity)))

	// Finally, we write our barometer reading, converted to tenths of millibars
	buffer.WriteString((fmt.Sprintf("b%05d", int64(a.currentReading.r.Barometer*33.8638866666667*10))))

	buffer.WriteString("." + "remoteweather-" + constants.Version)

	return buffer.String()
}

func convertLongitudeToAPRSFormat(l float64) string {
	var hemisphere string

	degrees := int(math.Floor(math.Abs(l)))
	remainder := math.Abs(l) - math.Floor(math.Abs(l))
	minutes := remainder * 60

	if l < 0 {
		hemisphere = "W"
	} else {
		hemisphere = "E"
	}

	return fmt.Sprintf("%03d%2.2f%v", degrees, minutes, hemisphere)
}

func convertLatitudeToAPRSFormat(l float64) string {
	var hemisphere string

	degrees := int(math.Floor(math.Abs(l)))
	remainder := math.Abs(l) - math.Floor(math.Abs(l))
	minutes := remainder * 60

	if l < 0 {
		hemisphere = "S"
	} else {
		hemisphere = "N"
	}

	return fmt.Sprintf("%2d%2.2f%v", degrees, minutes, hemisphere)
}

// AltitudeCompress generates a compressed altitude string for a given altitude (in feet)
func AltitudeCompress(a float64) []byte {
	var buffer bytes.Buffer

	// Altitude is compressed with the exponential equation:
	//   a = 1.002 ^ x
	//  where:
	//     a == altitude
	//     x == our pre-compressed altitude, to be converted to Base91
	precompAlt := int((math.Log(a) / math.Log(1.002)) + 0.5)

	// Convert our pre-compressed altitude to funky APRS-style Base91
	s := byte(precompAlt%91) + 33
	c := byte(precompAlt/91) + 33
	buffer.WriteByte(c)
	buffer.WriteByte(s)

	return buffer.Bytes()
}

// CourseCompress generates a compressed course byte for a given course (in degrees)
func CourseCompress(c int) byte {
	// Course is compressed with the equation:
	//   c = (x - 33) * 4
	//  where:
	//   c == course in degrees
	//   x == Keycode of compressed ASCII representation of course
	//
	//  So, to determine the correct ASCII keycode, we use this equivalent:
	//
	//  x = (c/4) + 33

	return byte(int(math.Floor((float64(c)/4)+.5) + 33))
}

// SpeedCompress generates a compressed speed byte for a given speed (in knots)
func SpeedCompress(s float64) byte {
	// Speed is compressed with the exponential equation:
	//   s = (1.08 ^ (x-33)) - 1
	// where:
	//      s == speed, in knots
	//      x == Keycode of compressed ASCII representation of speed
	//
	// So, to determine the correct ASCII keycode, we use this equivalent:
	// x = rnd(log(s) / log(1.08)) + 32

	// If the speed is 1 kt or less, just return ASCII 33
	if s <= 1 {
		return byte(33)
	}

	asciiVal := int(round(math.Log(s)/math.Log(1.08))) + 34
	return byte(asciiVal)
}

// LatPrecompress prepares a latitude (in decimal degrees) for Base91 conversion/compression
func LatPrecompress(l float64) float64 {

	// Formula for pre-compression of latitude, prior to Base91 conversion
	p := 380926 * (90 - l)
	return p
}

// LonPrecompress prepares a longitude (in decimal degrees) for Base91 conversion/compression
func LonPrecompress(l float64) float64 {

	// Formula for pre-compression of longitude, prior to Base91 conversion
	p := 190463 * (180 + l)
	return p
}

// EncodeBase91Position encodes a position to Base91 format
func EncodeBase91Position(l int) []byte {
	b91 := make([]byte, 4)
	p1Div := int(l / (91 * 91 * 91))
	p1Rem := l % (91 * 91 * 91)
	p2Div := int(p1Rem / (91 * 91))
	p2Rem := p1Rem % (91 * 91)
	p3Div := int(p2Rem / 91)
	p3Rem := p2Rem % 91
	b91[0] = byte(p1Div) + 33
	b91[1] = byte(p2Div) + 33
	b91[2] = byte(p3Div) + 33
	b91[3] = byte(p3Rem) + 33
	return b91
}

// EncodeBase91Telemetry encodes telemetry to Base91 format
func EncodeBase91Telemetry(l uint16) ([]byte, error) {

	if l > 8280 {
		return nil, errors.New("cannot encode telemetry value larger than 8280")
	}

	b91 := make([]byte, 2)
	p1Div := int(l / 91)
	p1Rem := l % 91
	b91[0] = byte(p1Div) + 33
	b91[1] = byte(p1Rem) + 33
	return b91, nil
}

//lint:ignore U1000 For future use
func mphToKnots(m float64) float64 {
	return m * 0.8689758
}

func round(x float64) float64 {
	if x > 0 {
		return math.Floor(x + 0.5)
	}
	return math.Ceil(x - 0.5)
}
