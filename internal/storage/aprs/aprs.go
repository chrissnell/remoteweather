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
	cfg             *types.Config
	APRSReadingChan chan types.Reading
	currentReading  *CurrentReading
}

// New sets up a new APRS-IS storage backend
func New(c *types.Config) (Storage, error) {
	a := Storage{}

	if c.Storage.APRS.Callsign == "" {
		return a, fmt.Errorf("you must provide a callsign in the configuration file")
	}

	if c.Storage.APRS.Location.Lat == 0 && c.Storage.APRS.Location.Lon == 0 {
		return a, fmt.Errorf("you must provide a latitude and longitude for your station in the configuration file")
	}

	if c.Storage.APRS.Passcode == "" {
		return a, fmt.Errorf("you must provide an APRS-IS passcode in the configuration file")
	}

	if c.Storage.APRS.APRSISServer == "" {
		c.Storage.APRS.APRSISServer = "noam.aprs2.net:14580"
	}

	a.cfg = c

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

	dialer := net.Dialer{
		Timeout: connectionTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", a.cfg.Storage.APRS.APRSISServer)
	if err != nil {
		log.Error("error dialing APRS-IS server %v: %v",
			a.cfg.Storage.APRS.APRSISServer, err)
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

	login := fmt.Sprintf("user %v pass %v vers remoteweather-%v\r\n",
		a.cfg.Storage.APRS.Callsign, a.cfg.Storage.APRS.Passcode, constants.Version)

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
func (a *Storage) CreateCompleteWeatherReport(symbolTable byte, symbol byte) string {
	a.currentReading.RLock()
	defer a.currentReading.RUnlock()

	if a.currentReading.r.Timestamp.Unix() == 0 {
		return ""
	}

	var b bytes.Buffer

	// Create callsign field
	b.WriteString(a.cfg.Storage.APRS.Callsign)
	b.WriteString(">APRS,TCPIP*:")

	// Create position timestamp
	b.WriteString(a.currentReading.r.Timestamp.Format("021504z"))

	// Create the compressed position report
	lat := LatPrecompress(a.cfg.Storage.APRS.Location.Lat)
	lon := LonPrecompress(a.cfg.Storage.APRS.Location.Lon)

	latBytes := EncodeBase91Position(int(lat))
	lonBytes := EncodeBase91Position(int(lon))

	b.WriteByte(symbolTable)
	b.Write(latBytes)
	b.Write(lonBytes)
	b.WriteByte(symbol)

	var cse int
	if !math.IsNaN(float64(a.currentReading.r.WindDir)) {
		cse = int(a.currentReading.r.WindDir)
	}

	var spd float64
	if !math.IsNaN(float64(a.currentReading.r.WindSpeed)) {
		spd = mphToKnots(float64(a.currentReading.r.WindSpeed))
	}

	b.WriteByte(CourseCompress(cse))
	b.WriteByte(SpeedCompress(spd))

	// Add the station type identifier
	b.WriteByte(byte(98))

	// Weather report
	b.WriteString("_")

	// Timestamp
	b.WriteString(a.currentReading.r.Timestamp.Format("150405"))

	// Wind direction and speed
	if !math.IsNaN(float64(a.currentReading.r.WindDir)) {
		b.WriteString(fmt.Sprintf("c%s", convertWindDirToAPRSFormat(float64(a.currentReading.r.WindDir))))
	} else {
		b.WriteString("c...")
	}

	if !math.IsNaN(float64(a.currentReading.r.WindSpeed)) {
		b.WriteString(fmt.Sprintf("s%s", convertWindSpeedToAPRSFormat(mphToKnots(float64(a.currentReading.r.WindSpeed)))))
	} else {
		b.WriteString("s...")
	}

	// Gust (we don't have this, so we skip it)
	b.WriteString("g...")

	// Temperature
	if !math.IsNaN(float64(a.currentReading.r.OutTemp)) {
		b.WriteString(fmt.Sprintf("t%s", convertTempToAPRSFormat(float64(a.currentReading.r.OutTemp))))
	} else {
		b.WriteString("t...")
	}

	// Rainfall in last hour
	b.WriteString("r...")

	// Rainfall in last 24 hours
	b.WriteString("p...")

	// Rainfall since midnight
	if !math.IsNaN(float64(a.currentReading.r.DayRain)) {
		b.WriteString(fmt.Sprintf("P%s", convertRainToAPRSFormat(float64(a.currentReading.r.DayRain))))
	} else {
		b.WriteString("P...")
	}

	// Humidity
	if !math.IsNaN(float64(a.currentReading.r.OutHumidity)) {
		b.WriteString(fmt.Sprintf("h%s", convertHumidityToAPRSFormat(float64(a.currentReading.r.OutHumidity))))
	} else {
		b.WriteString("h..")
	}

	// Barometric pressure
	if !math.IsNaN(float64(a.currentReading.r.Barometer)) {
		b.WriteString(fmt.Sprintf("b%s", convertBarometerToAPRSFormat(float64(a.currentReading.r.Barometer))))
	} else {
		b.WriteString("b.....")
	}

	return b.String()
}

func convertWindDirToAPRSFormat(w float64) string {
	return fmt.Sprintf("%03.0f", w)
}

func convertWindSpeedToAPRSFormat(w float64) string {
	return fmt.Sprintf("%03.0f", w)
}

func convertTempToAPRSFormat(t float64) string {
	return fmt.Sprintf("%03.0f", t)
}

func convertRainToAPRSFormat(r float64) string {
	return fmt.Sprintf("%03.0f", r*100)
}

func convertHumidityToAPRSFormat(h float64) string {
	if h == 100.0 {
		return "00"
	}
	return fmt.Sprintf("%02.0f", h)
}

func convertBarometerToAPRSFormat(b float64) string {
	return fmt.Sprintf("%05.0f", b*10)
}

func convertLongitudeToAPRSFormat(l float64) string {
	degrees := int(math.Abs(l))
	minutes := (math.Abs(l) - float64(degrees)) * 60

	var direction string
	if l < 0 {
		direction = "W"
	} else {
		direction = "E"
	}

	return fmt.Sprintf("%03d%05.2f%s", degrees, minutes, direction)
}

func convertLatitudeToAPRSFormat(l float64) string {
	degrees := int(math.Abs(l))
	minutes := (math.Abs(l) - float64(degrees)) * 60

	var direction string
	if l < 0 {
		direction = "S"
	} else {
		direction = "N"
	}

	return fmt.Sprintf("%02d%05.2f%s", degrees, minutes, direction)
}

func AltitudeCompress(a float64) []byte {
	var altBytes []byte
	cs := int(math.Log(a) / math.Log(1.002))
	altBytes = EncodeBase91Position(cs)
	return altBytes
}

func CourseCompress(c int) byte {
	return byte(c/4) + 33
}

func SpeedCompress(s float64) byte {
	// Speed is in knots
	return byte(math.Log(s+1)/math.Log(1.08)) + 33
}

func LatPrecompress(l float64) float64 {
	return 380926 * (90 - l)
}

func LonPrecompress(l float64) float64 {
	return 190463 * (180 + l)
}

func EncodeBase91Position(l int) []byte {
	var encBytes []byte
	encBytes = append(encBytes, byte(l/753571)+33)
	encBytes = append(encBytes, byte((l%753571)/8281)+33)
	encBytes = append(encBytes, byte(((l%753571)%8281)/91)+33)
	encBytes = append(encBytes, byte(((l%753571)%8281)%91)+33)
	return encBytes
}

func EncodeBase91Telemetry(l uint16) ([]byte, error) {
	var encBytes []byte

	if l > 8280 {
		return encBytes, errors.New("telemetry value too large for base-91 encoding")
	}

	encBytes = append(encBytes, byte(l/91)+33)
	encBytes = append(encBytes, byte(l%91)+33)
	return encBytes, nil
}

func mphToKnots(m float64) float64 {
	return m * 0.868976
}

func round(x float64) float64 {
	return math.Floor(x + 0.5)
}
