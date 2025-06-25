package storage

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

// APRSStorage holds general configuration related to our APRS/CWOP transmissions
type APRSStorage struct {
	ctx             context.Context
	cfg             *types.Config
	APRSReadingChan chan types.Reading
	currentReading  *CurrentReading
}

// NewAPRSStorage sets up a new APRS-IS storage backend
func NewAPRSStorage(c *types.Config) (APRSStorage, error) {
	a := APRSStorage{}

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
func (a APRSStorage) StartStorageEngine(ctx context.Context, wg *sync.WaitGroup) chan<- types.Reading {
	log.Info("starting APRS-IS storage engine...")
	a.ctx = ctx
	readingChan := make(chan types.Reading)

	a.currentReading = &CurrentReading{}
	a.currentReading.r = types.Reading{}
	go a.processMetrics(ctx, wg, readingChan)
	go a.sendReports(ctx, wg)
	return readingChan
}

func (a *APRSStorage) sendReports(ctx context.Context, wg *sync.WaitGroup) {
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

func (a *APRSStorage) sendReadingToAPRSIS(ctx context.Context, wg *sync.WaitGroup) {
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

func (a *APRSStorage) processMetrics(ctx context.Context, wg *sync.WaitGroup, rchan <-chan types.Reading) {
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
func (a *APRSStorage) StoreCurrentReading(r types.Reading) error {
	a.currentReading.Lock()
	a.currentReading.r = r
	a.currentReading.Unlock()
	return nil
}

// CreateCompleteWeatherReport creates an APRS weather report with compressed position
// report included.
func (a *APRSStorage) CreateCompleteWeatherReport(symbolTable byte, symbol byte) string {
	// Lock our mutex for reading
	a.currentReading.RLock()

	// Build compressed position report
	var buffer bytes.Buffer

	// Send out callsign
	buffer.WriteString(a.cfg.Storage.APRS.Callsign)

	// Begin our APRS data
	buffer.WriteString(">APRS:!")

	lat := LatPrecompress(a.cfg.Storage.APRS.Location.Lat)
	lon := LonPrecompress(a.cfg.Storage.APRS.Location.Lon)

	// Encode lat/lon per APRS spec
	latBytes := EncodeBase91Position(int(lat))
	lonBytes := EncodeBase91Position(int(lon))

	// Write the bytes to our buffer
	buffer.Write(latBytes)
	buffer.Write(lonBytes)

	// Write symbol table
	buffer.WriteByte(symbolTable)

	// Calculate our wind direction and speed bytes
	// If we have zero wind speed, then we send wind direction of 0,
	// regardless of what the wind vane is showing
	var windDirCompass, windSpeed int
	if a.currentReading.r.WindSpeed != 0 {
		windDirCompass = int(a.currentReading.r.WindDir)
		windSpeed = int(a.currentReading.r.WindSpeed)
	} else {
		windDirCompass = 0
		windSpeed = 0
	}

	// Generate our course/speed bytes and write them to the buffer
	buffer.WriteByte(CourseCompress(windDirCompass))
	buffer.WriteByte(SpeedCompress(float64(windSpeed)))

	// Write symbol ID
	buffer.WriteByte(symbol)

	// Then we add our temperature reading
	buffer.WriteString(fmt.Sprintf("t%03d", int64(a.currentReading.r.OutTemp)))

	// then we add our rainfall reading for the past 24hrs, in hundredths of an inch
	buffer.WriteString(fmt.Sprintf("P%03d", int64(a.currentReading.r.DayRain*100)))

	// then we write our humidity reading
	buffer.WriteString(fmt.Sprintf("h%02d", int64(a.currentReading.r.OutHumidity)))

	// Finally, we write our barometer reading, converted to tenths of millibars
	buffer.WriteString((fmt.Sprintf("b%05d", int64(a.currentReading.r.Barometer*33.8638866666667*10))))

	// End critical section
	a.currentReading.RUnlock()

	return buffer.String()
}

func convertLongitudeToAPRSFormat(l float64) string {
	var dir byte
	if l < 0 {
		dir = 'W'
		l = math.Abs(l)
	} else {
		dir = 'E'
	}

	degrees := int(l)
	minutes := (l - float64(degrees)) * 60

	return fmt.Sprintf("%03d%05.2f%c", degrees, minutes, dir)

}

func convertLatitudeToAPRSFormat(l float64) string {
	var dir byte
	if l < 0 {
		dir = 'S'
		l = math.Abs(l)
	} else {
		dir = 'N'
	}

	degrees := int(l)
	minutes := (l - float64(degrees)) * 60

	return fmt.Sprintf("%02d%05.2f%c", degrees, minutes, dir)

}

func AltitudeCompress(a float64) []byte {
	compressed := make([]byte, 2)

	s := int(math.Log(a*3.28084) / math.Log(1.002))

	compressed[0] = byte(s/91) + 33
	compressed[1] = byte(s%91) + 33

	return compressed
}

func CourseCompress(c int) byte {
	if c == 0 {
		return byte(0 + 33)
	}

	if c == 360 {
		c = 0
	}

	return byte(c/4 + 33)
}

func SpeedCompress(s float64) byte {
	kts := mphToKnots(s)

	// APRS spec says we should do this
	compressed := int(math.Log(kts+1)/math.Log(1.08) + 0.5)

	if compressed > 90 {
		compressed = 90
	}

	return byte(compressed + 33)
}

func LatPrecompress(l float64) float64 {

	return 380926 * (90 - l)
}

func LonPrecompress(l float64) float64 {

	return 190463 * (180 + l)
}

func EncodeBase91Position(l int) []byte {
	encoded := make([]byte, 4)

	encoded[0] = byte(l/(91*91*91)) + 33
	encoded[1] = byte((l/(91*91))%91) + 33
	encoded[2] = byte((l/91)%91) + 33
	encoded[3] = byte(l%91) + 33

	return encoded

}

func EncodeBase91Telemetry(l uint16) ([]byte, error) {
	if l > 8280 {
		return nil, errors.New("EncodeBase91Telemetry: argument too large to encode")
	}

	encoded := make([]byte, 2)

	encoded[0] = byte(int(l)/91) + 33
	encoded[1] = byte(int(l)%91) + 33

	return encoded, nil
}

func mphToKnots(m float64) float64 {
	return m * 0.868976
}

func round(x float64) float64 {
	return float64(int(x + 0.5))
}
