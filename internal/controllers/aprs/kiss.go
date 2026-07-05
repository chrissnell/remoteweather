package aprs

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/kiss"
	serial "github.com/tarm/goserial"
)

const (
	transportKISS = "kiss"

	kissConnectionSerial = "serial"
	kissConnectionTCP    = "tcp"

	defaultKISSBaud        = 9600
	defaultKISSPath        = "WIDE1-1,WIDE2-1"
	defaultKISSDestination = "APRS"
)

// sendReadingViaKISS builds an APRS weather report, encodes it as an AX.25 UI
// frame wrapped in KISS, and sends it to the configured TNC (serial or network).
func (a *Controller) sendReadingViaKISS(ctx context.Context, device config.DeviceData, reading database.FetchedBucketReading, windGust float32) {
	info := a.buildWeatherReportInfo(device, reading, windGust, '/', '_')

	dest := device.APRSKISSDestination
	if dest == "" {
		dest = defaultKISSDestination
	}

	pathStr := device.APRSKISSPath
	if pathStr == "" {
		pathStr = defaultKISSPath
	}
	path := splitPath(pathStr)

	frame, err := kiss.EncodeAX25UI(device.APRSCallsign, dest, path, []byte(info))
	if err != nil {
		log.Errorf("error building AX.25 frame for %s: %v", device.Name, err)
		return
	}
	kissFrame := kiss.EncodeFrame(frame)

	log.Debugf("sending KISS frame for station %s (%d bytes) via %s", device.Name, len(kissFrame), device.APRSKISSConnection)

	if err := a.sendKISSFrame(ctx, device, kissFrame); err != nil {
		log.Errorf("error sending KISS frame for %s: %v", device.Name, err)
	}
}

// sendKISSFrame opens the device's configured KISS connection, writes the frame,
// and closes it. Connections are opened per report to match the APRS-IS path.
func (a *Controller) sendKISSFrame(ctx context.Context, device config.DeviceData, frame []byte) error {
	conn, err := openKISSConnection(ctx, device)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write(frame); err != nil {
		return fmt.Errorf("failed to write KISS frame: %w", err)
	}
	return nil
}

// validateKISSConfig reports whether a device's KISS connection settings are
// complete enough to open a connection, without performing any I/O. It is used
// both by the health monitor and as a precondition in openKISSConnection.
func validateKISSConfig(device config.DeviceData) error {
	switch strings.ToLower(device.APRSKISSConnection) {
	case kissConnectionSerial:
		if device.APRSKISSSerialDevice == "" {
			return fmt.Errorf("KISS serial connection requires aprs_kiss_serial_device")
		}
	case kissConnectionTCP:
		if device.APRSKISSTCPAddress == "" {
			return fmt.Errorf("KISS tcp connection requires aprs_kiss_tcp_address")
		}
	default:
		return fmt.Errorf("invalid KISS connection type %q (must be %q or %q)",
			device.APRSKISSConnection, kissConnectionSerial, kissConnectionTCP)
	}
	return nil
}

// openKISSConnection dials the TNC over serial or TCP based on device config.
func openKISSConnection(ctx context.Context, device config.DeviceData) (io.ReadWriteCloser, error) {
	if err := validateKISSConfig(device); err != nil {
		return nil, err
	}

	switch strings.ToLower(device.APRSKISSConnection) {
	case kissConnectionSerial:
		baud := device.APRSKISSSerialBaud
		if baud <= 0 {
			baud = defaultKISSBaud
		}
		cfg := &serial.Config{Name: device.APRSKISSSerialDevice, Baud: baud}
		port, err := serial.OpenPort(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to open serial port %s: %w", device.APRSKISSSerialDevice, err)
		}
		return port, nil

	default: // kissConnectionTCP (validated above)
		dialer := net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", device.APRSKISSTCPAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to TNC %s: %w", device.APRSKISSTCPAddress, err)
		}
		return conn, nil
	}
}

// splitPath parses a comma-separated AX.25 digipeater path into hops.
func splitPath(path string) []string {
	var hops []string
	for _, h := range strings.Split(path, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hops = append(hops, h)
		}
	}
	return hops
}
