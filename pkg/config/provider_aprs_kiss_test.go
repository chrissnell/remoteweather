package config

import "testing"

func TestDeviceAPRSKISSRoundTrip(t *testing.T) {
	p := newTestProvider(t)

	dev := &DeviceData{
		Name:                 "kiss-station",
		Type:                 "campbellscientific",
		Enabled:              true,
		APRSEnabled:          true,
		APRSCallsign:         "N0CALL-13",
		APRSTransport:        "kiss",
		APRSKISSConnection:   "tcp",
		APRSKISSTCPAddress:   "127.0.0.1:8001",
		APRSKISSSerialDevice: "/dev/ttyUSB0",
		APRSKISSSerialBaud:   19200,
		APRSKISSPath:         "WIDE1-1,WIDE2-1",
		APRSKISSDestination:  "APZ001",
	}
	if err := p.AddDevice(dev); err != nil {
		t.Fatalf("AddDevice: %v", err)
	}

	got, err := p.GetDevice("kiss-station")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}

	checks := []struct {
		name      string
		got, want string
	}{
		{"transport", got.APRSTransport, "kiss"},
		{"connection", got.APRSKISSConnection, "tcp"},
		{"tcp_address", got.APRSKISSTCPAddress, "127.0.0.1:8001"},
		{"serial_device", got.APRSKISSSerialDevice, "/dev/ttyUSB0"},
		{"path", got.APRSKISSPath, "WIDE1-1,WIDE2-1"},
		{"destination", got.APRSKISSDestination, "APZ001"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if got.APRSKISSSerialBaud != 19200 {
		t.Errorf("serial_baud = %d, want 19200", got.APRSKISSSerialBaud)
	}

	// Update the transport back to APRS-IS and confirm it persists.
	got.APRSTransport = "aprs-is"
	got.APRSKISSConnection = "serial"
	if err := p.UpdateDevice("kiss-station", got); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}
	after, err := p.GetDevice("kiss-station")
	if err != nil {
		t.Fatalf("GetDevice after update: %v", err)
	}
	if after.APRSTransport != "aprs-is" {
		t.Errorf("after update transport = %q, want aprs-is", after.APRSTransport)
	}
	if after.APRSKISSConnection != "serial" {
		t.Errorf("after update connection = %q, want serial", after.APRSKISSConnection)
	}
}

func TestDeviceAPRSKISSDefaults(t *testing.T) {
	p := newTestProvider(t)

	// A device added without an explicit transport is normalized to aprs-is so
	// the stored value is always concrete. KISS-specific fields stay empty.
	dev := &DeviceData{Name: "plain", Type: "campbellscientific", Enabled: true}
	if err := p.AddDevice(dev); err != nil {
		t.Fatalf("AddDevice: %v", err)
	}
	got, err := p.GetDevice("plain")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.APRSTransport != "aprs-is" {
		t.Errorf("default transport = %q, want aprs-is", got.APRSTransport)
	}
	if got.APRSKISSConnection != "" {
		t.Errorf("default connection = %q, want empty", got.APRSKISSConnection)
	}
}
