package aprs

import (
	"testing"

	"github.com/chrissnell/remoteweather/pkg/config"
)

func TestValidateKISSConfig(t *testing.T) {
	cases := []struct {
		name    string
		device  config.DeviceData
		wantErr bool
	}{
		{
			name:    "valid serial",
			device:  config.DeviceData{APRSKISSConnection: "serial", APRSKISSSerialDevice: "/dev/ttyUSB0"},
			wantErr: false,
		},
		{
			name:    "serial missing device",
			device:  config.DeviceData{APRSKISSConnection: "serial"},
			wantErr: true,
		},
		{
			name:    "valid tcp",
			device:  config.DeviceData{APRSKISSConnection: "tcp", APRSKISSTCPAddress: "127.0.0.1:8001"},
			wantErr: false,
		},
		{
			name:    "tcp missing address",
			device:  config.DeviceData{APRSKISSConnection: "tcp"},
			wantErr: true,
		},
		{
			name:    "empty connection type",
			device:  config.DeviceData{},
			wantErr: true,
		},
		{
			name:    "unknown connection type",
			device:  config.DeviceData{APRSKISSConnection: "usb"},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateKISSConfig(c.device)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	got := splitPath("WIDE1-1, WIDE2-1 ,")
	want := []string{"WIDE1-1", "WIDE2-1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hop %d = %q, want %q", i, got[i], want[i])
		}
	}
}
