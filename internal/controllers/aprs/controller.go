// Package aprs provides APRS/CWOP controller for transmitting weather data to amateur radio networks.
package aprs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/internal/constants"
	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	aprspkg "github.com/chrissnell/remoteweather/pkg/aprs"
	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// Controller holds general configuration related to our APRS/CWOP transmissions
type Controller struct {
	ctx              context.Context
	cancel           context.CancelFunc
	configProvider   config.ConfigProvider
	DB               *database.Client
	wg               *sync.WaitGroup
	logger           *zap.SugaredLogger
	running          bool
	runningMutex     sync.RWMutex
}

// New creates a new APRS controller
func New(configProvider config.ConfigProvider) (*Controller, error) {
	// Validate TimescaleDB configuration
	if err := controllers.ValidateTimescaleDBConfig(configProvider, "APRS"); err != nil {
		return nil, err
	}

	// Set up database connection
	db, err := controllers.SetupDatabaseConnection(configProvider, nil)
	if err != nil {
		return nil, err
	}

	a := &Controller{
		configProvider: configProvider,
		DB:             db,
		wg:             &sync.WaitGroup{},
	}

	// Load all controllers to find APRS controller configuration
	cfg, err := configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	// Find APRS controller configuration
	var aprsConfig *config.APRSData
	for _, controller := range cfg.Controllers {
		if controller.Type == "aprs" && controller.APRS != nil {
			aprsConfig = controller.APRS
			break
		}
	}

	if aprsConfig == nil || aprsConfig.Server == "" {
		return nil, fmt.Errorf("APRS controller configuration is missing or incomplete")
	}

	// Check if we have at least one device with APRS enabled
	devices, err := configProvider.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("error loading device configurations: %v", err)
	}

	// Validate at least one device has APRS enabled with callsign and location
	validStation := false
	for _, device := range devices {
		if device.APRSEnabled && device.APRSCallsign != "" &&
			device.Latitude != 0 && device.Longitude != 0 {
			validStation = true
			break
		}
	}

	if !validStation {
		return nil, fmt.Errorf("you must configure at least one weather station with APRS enabled, callsign, and location")
	}

	return a, nil
}

// StartController starts the APRS controller (controller manager interface)
func (a *Controller) StartController() error {
	a.runningMutex.Lock()
	defer a.runningMutex.Unlock()

	if a.running {
		return fmt.Errorf("APRS controller is already running")
	}

	log.Info("Starting APRS controller...")
	a.ctx, a.cancel = context.WithCancel(context.Background())
	
	// Start sending reports for all APRS-enabled devices
	go a.sendReports(a.ctx, a.wg)

	// Start health monitoring
	log.Info("starting APRS health monitor")
	a.startHealthMonitor(a.ctx, a.configProvider)

	a.running = true
	log.Info("APRS controller started")
	return nil
}

// Start starts the APRS controller
func (a *Controller) Start(ctx context.Context) error {
	a.runningMutex.Lock()
	defer a.runningMutex.Unlock()

	if a.running {
		return fmt.Errorf("APRS controller is already running")
	}

	log.Info("Starting APRS controller...")
	a.ctx, a.cancel = context.WithCancel(ctx)
	
	// Start sending reports for all APRS-enabled devices
	go a.sendReports(a.ctx, a.wg)

	// Start health monitoring
	log.Info("starting APRS health monitor")
	a.startHealthMonitor(a.ctx, a.configProvider)

	a.running = true
	log.Info("APRS controller started")
	return nil
}

// Stop stops the APRS controller
func (a *Controller) Stop() error {
	a.runningMutex.Lock()
	defer a.runningMutex.Unlock()

	if !a.running {
		return fmt.Errorf("APRS controller is not running")
	}

	log.Info("Stopping APRS controller...")
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	a.running = false
	log.Info("APRS controller stopped")
	return nil
}

// IsRunning returns whether the controller is running
func (a *Controller) IsRunning() bool {
	a.runningMutex.RLock()
	defer a.runningMutex.RUnlock()
	return a.running
}

// GetHealth returns the health status of the controller
func (a *Controller) GetHealth() map[string]interface{} {
	health := map[string]interface{}{
		"name":    "APRS",
		"running": a.IsRunning(),
	}

	// Get health from config provider if available
	if a.configProvider != nil {
		if healthData, err := a.configProvider.GetStorageHealth("aprs"); err == nil {
			health["status"] = healthData.Status
			health["message"] = healthData.Message
			health["last_check"] = healthData.LastCheck
			if healthData.Error != "" {
				health["error"] = healthData.Error
			}
		}
	}

	return health
}

// getAPRSConfig retrieves the APRS controller configuration
func (a *Controller) getAPRSConfig() (*config.APRSData, error) {
	cfg, err := a.configProvider.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}

	for _, controller := range cfg.Controllers {
		if controller.Type == "aprs" && controller.APRS != nil {
			return controller.APRS, nil
		}
	}

	return nil, fmt.Errorf("APRS controller configuration not found")
}

func (a *Controller) sendReports(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	// Get all APRS-enabled devices
	devices, err := a.configProvider.GetDevices()
	if err != nil {
		log.Errorf("Error getting devices: %v", err)
		return
	}

	// Start a goroutine for each APRS-enabled device
	for _, device := range devices {
		if device.APRSEnabled && device.APRSCallsign != "" && 
			device.Latitude != 0 && device.Longitude != 0 {
			// Create a copy for the closure
			deviceCopy := device
			
			log.Infof("Starting APRS reporting for device: %s (Callsign: %s)", 
				device.Name, device.APRSCallsign)
			
			// Start monitoring in separate goroutine
			go a.sendDeviceReports(ctx, wg, deviceCopy)
		}
	}
}

func (a *Controller) sendDeviceReports(ctx context.Context, wg *sync.WaitGroup, device config.DeviceData) {
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Send initial report after 15 seconds
	initialTimer := time.NewTimer(15 * time.Second)
	select {
	case <-initialTimer.C:
		log.Debugf("Sending initial APRS report for %s", device.Name)
		a.sendStationReadingToAPRSIS(ctx, wg, device)
	case <-ctx.Done():
		initialTimer.Stop()
		return
	}

	// Continue with regular reports
	for {
		select {
		case <-ticker.C:
			log.Debugf("Sending APRS report for %s", device.Name)
			a.sendStationReadingToAPRSIS(ctx, wg, device)
		case <-ctx.Done():
			log.Infof("Stopping APRS reports for %s", device.Name)
			return
		}
	}
}

func (a *Controller) sendStationReadingToAPRSIS(ctx context.Context, wg *sync.WaitGroup, device config.DeviceData) {
	wg.Add(1)
	defer wg.Done()

	// Get latest reading from database
	reading, err := a.DB.GetReadingsFromTimescaleDB(device.Name)
	if err != nil {
		log.Errorf("Error getting reading for %s: %v", device.Name, err)
		return
	}

	connectionTimeout := 3 * time.Second

	pkt := a.CreateCompleteWeatherReport(device, reading, '/', '_')
	log.Debugf("sending reading to APRS-IS for station %s: %+v", device.Name, pkt)

	// Load APRS controller configuration
	aprsConfig, err := a.getAPRSConfig()
	if err != nil {
		log.Error("error loading APRS controller configuration: %v", err)
		return
	}


	dialer := net.Dialer{
		Timeout: connectionTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", aprsConfig.Server)
	if err != nil {
		log.Error("error dialing APRS-IS server %v: %v",
			aprsConfig.Server, err)
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

	// Calculate passcode from callsign or use device-specific passcode
	var passcode int
	if device.APRSPasscode != "" {
		// Try to parse the passcode as integer
		if p, err := strconv.Atoi(device.APRSPasscode); err == nil {
			passcode = p
		} else {
			// Fall back to calculated passcode
			passcode = aprspkg.CalculatePasscode(device.APRSCallsign)
		}
	} else {
		passcode = aprspkg.CalculatePasscode(device.APRSCallsign)
	}

	login := fmt.Sprintf("user %v pass %v vers remoteweather-%v\r\n",
		device.APRSCallsign, passcode, constants.Version)

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


// CreateCompleteWeatherReport creates an APRS weather report with compressed position
// report included.
func (a *Controller) CreateCompleteWeatherReport(device config.DeviceData, reading database.FetchedBucketReading, symTable, symCode rune) string {
	var buffer bytes.Buffer

	// Build callsign and position
	callsign := strings.ToUpper(device.APRSCallsign)

	// Use device-specific symbol table/code if available
	if device.APRSSymbolTable != "" && len(device.APRSSymbolTable) > 0 {
		symTable = rune(device.APRSSymbolTable[0])
	}
	if device.APRSSymbolCode != "" && len(device.APRSSymbolCode) > 0 {
		symCode = rune(device.APRSSymbolCode[0])
	}

	latAPRS := convertLatitudeToAPRSFormat(device.Latitude)
	lonAPRS := convertLongitudeToAPRSFormat(device.Longitude)

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
	buffer.WriteString(fmt.Sprintf("%03d/%03d", int(reading.WindDir), int(reading.WindSpeed)))

	// We don't keep track of gusts
	buffer.WriteString("g...")

	// Then we add our temperature reading
	buffer.WriteString(fmt.Sprintf("t%03d", int64(reading.OutTemp)))

	// Then we add our rainfall since midnight
	buffer.WriteString(fmt.Sprintf("P%03d", int64(reading.DayRain*100)))

	// Then we add our humidity
	buffer.WriteString(fmt.Sprintf("h%02d", int64(reading.OutHumidity)))

	// Finally, we write our barometer reading, converted to tenths of millibars
	buffer.WriteString((fmt.Sprintf("b%05d", int64(reading.Barometer*33.8638866666667*10))))

	buffer.WriteString("." + "remoteweather-" + constants.Version)
	
	// Add device-specific comment if configured
	if device.APRSComment != "" {
		buffer.WriteString(" " + device.APRSComment)
	}

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

func round(x float64) float64 {
	if x > 0 {
		return math.Floor(x + 0.5)
	}
	return math.Ceil(x - 0.5)
}

// startHealthMonitor starts a goroutine that periodically updates the health status
func (a *Controller) startHealthMonitor(ctx context.Context, configProvider config.ConfigProvider) {
	go func() {
		// Run initial health check immediately
		a.updateHealthStatus(configProvider)

		ticker := time.NewTicker(90 * time.Second) // Update health every 90 seconds (less frequent due to network calls)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.updateHealthStatus(configProvider)
			case <-ctx.Done():
				log.Info("stopping APRS health monitor")
				return
			}
		}
	}()
}

// updateHealthStatus performs a health check and updates the status in the config database
func (a *Controller) updateHealthStatus(configProvider config.ConfigProvider) {
	health := &config.StorageHealthData{
		LastCheck: time.Now(),
		Status:    "healthy",
		Message:   "APRS-IS connection available",
	}

	// Test APRS-IS server connectivity and authentication for all enabled devices
	devices, err := configProvider.GetDevices()
	if err != nil {
		health.Status = "unhealthy"
		health.Message = "Failed to load device configurations"
		health.Error = err.Error()
	} else {
		// Count enabled devices and test first one
		enabledCount := 0
		var firstDevice *config.DeviceData
		for _, device := range devices {
			if device.APRSEnabled && device.APRSCallsign != "" {
				enabledCount++
				if firstDevice == nil {
					firstDevice = &device
				}
			}
		}
		
		if enabledCount == 0 {
			health.Status = "unhealthy"
			health.Message = "No APRS-enabled devices found"
		} else {
			// Test login with first device
			err := a.testAPRSISLogin(configProvider)
			if err != nil {
				health.Status = "unhealthy"
				health.Message = fmt.Sprintf("APRS-IS login test failed for %d enabled device(s)", enabledCount)
				health.Error = err.Error()
			} else {
				health.Status = "healthy"
				health.Message = fmt.Sprintf("APRS-IS login test successful (%d enabled device(s))", enabledCount)
			}
		}
	}

	// Update health status in configuration database
	err = configProvider.UpdateStorageHealth("aprs", health)
	if err != nil {
		log.Errorf("Failed to update APRS health status: %v", err)
	} else {
		log.Debugf("Updated APRS health status: %s", health.Status)
	}
}

// testAPRSISLogin performs a test login to the APRS-IS server to verify connectivity and authentication
func (a *Controller) testAPRSISLogin(configProvider config.ConfigProvider) error {
	connectionTimeout := 10 * time.Second

	// Load APRS controller configuration
	aprsConfig, err := a.getAPRSConfig()
	if err != nil {
		return fmt.Errorf("error loading APRS controller configuration: %v", err)
	}

	// Get devices with APRS enabled
	devices, err := configProvider.GetDevices()
	if err != nil {
		return fmt.Errorf("error loading device configurations: %v", err)
	}

	var aprsCallsign string
	for _, device := range devices {
		if device.APRSEnabled && device.APRSCallsign != "" {
			aprsCallsign = device.APRSCallsign
			break
		}
	}

	if aprsCallsign == "" {
		return fmt.Errorf("no enabled APRS device found")
	}

	// Test connection to APRS-IS server
	dialer := net.Dialer{
		Timeout: connectionTimeout,
	}

	conn, err := dialer.Dial("tcp", aprsConfig.Server)
	if err != nil {
		return fmt.Errorf("failed to connect to APRS-IS server %s: %v", aprsConfig.Server, err)
	}
	defer conn.Close()

	buffCon := bufio.NewReader(conn)

	// Set read deadline for server greeting
	conn.SetReadDeadline(time.Now().Add(connectionTimeout))

	// Read server greeting
	resp, err := buffCon.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read APRS-IS server greeting: %v", err)
	}

	// Verify proper greeting format
	if len(resp) == 0 || resp[0] != '#' {
		return fmt.Errorf("APRS-IS server responded with invalid greeting: %s", strings.TrimSpace(resp))
	}

	// Calculate passcode from callsign
	passcode := aprspkg.CalculatePasscode(aprsCallsign)

	// Send login command
	loginCmd := fmt.Sprintf("user %s pass %d vers remoteweather-healthcheck 1.0\r\n",
		aprsCallsign, passcode)

	conn.SetWriteDeadline(time.Now().Add(connectionTimeout))
	_, err = conn.Write([]byte(loginCmd))
	if err != nil {
		return fmt.Errorf("failed to send login command: %v", err)
	}

	// Read login response
	conn.SetReadDeadline(time.Now().Add(connectionTimeout))
	loginResp, err := buffCon.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read login response: %v", err)
	}

	// Check if login was successful
	// APRS-IS typically responds with a line starting with '#' containing "verified" for successful logins
	loginResp = strings.TrimSpace(loginResp)
	if !strings.Contains(strings.ToLower(loginResp), "verified") {
		return fmt.Errorf("APRS-IS login failed, server response: %s", loginResp)
	}

	log.Debugf("APRS-IS health check successful for callsign %s", aprsCallsign)
	return nil
}
