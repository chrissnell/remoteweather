package config

import "testing"

// TestDeviceServiceConfigRoundTrip verifies that per-device weather service
// configuration — including the configurable send/upload intervals — is
// persisted and read back through AddDevice/GetDevice and UpdateDevice.
func TestDeviceServiceConfigRoundTrip(t *testing.T) {
	p := newTestProvider(t)

	dev := &DeviceData{
		Name:                 "svc-station",
		Type:                 "campbellscientific",
		Enabled:              true,
		PWSEnabled:           true,
		PWSStationID:         "PWS123",
		PWSPassword:          "pws-secret",
		PWSUploadInterval:    120,
		PWSAPIEndpoint:       "https://pws.example.com/submit",
		WUEnabled:            true,
		WUStationID:          "WU456",
		WUPassword:           "wu-secret",
		WUUploadInterval:     600,
		WUAPIEndpoint:        "https://wu.example.com/submit",
		AerisEnabled:         true,
		AerisAPIClientID:     "aeris-id",
		AerisAPIClientSecret: "aeris-secret",
		AerisAPIEndpoint:     "https://aeris.example.com/",
		AerisRefreshInterval: 7200,
		APRSEnabled:          true,
		APRSCallsign:         "N0CALL",
		APRSUploadInterval:   900,
	}
	if err := p.AddDevice(dev); err != nil {
		t.Fatalf("AddDevice: %v", err)
	}

	got, err := p.GetDevice("svc-station")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}

	intChecks := []struct {
		name      string
		got, want int
	}{
		{"pws_upload_interval", got.PWSUploadInterval, 120},
		{"wu_upload_interval", got.WUUploadInterval, 600},
		{"aeris_refresh_interval", got.AerisRefreshInterval, 7200},
		{"aprs_upload_interval", got.APRSUploadInterval, 900},
	}
	for _, c := range intChecks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if !got.PWSEnabled || !got.WUEnabled || !got.AerisEnabled {
		t.Errorf("service enablement not persisted: pws=%v wu=%v aeris=%v", got.PWSEnabled, got.WUEnabled, got.AerisEnabled)
	}
	if got.PWSStationID != "PWS123" || got.WUStationID != "WU456" || got.AerisAPIClientID != "aeris-id" {
		t.Errorf("service credentials not persisted: %q %q %q", got.PWSStationID, got.WUStationID, got.AerisAPIClientID)
	}
	// GetDevice must return the *_api_endpoint columns, matching GetDevices.
	if got.PWSAPIEndpoint != "https://pws.example.com/submit" ||
		got.WUAPIEndpoint != "https://wu.example.com/submit" ||
		got.AerisAPIEndpoint != "https://aeris.example.com/" {
		t.Errorf("service API endpoints not returned by GetDevice: pws=%q wu=%q aeris=%q",
			got.PWSAPIEndpoint, got.WUAPIEndpoint, got.AerisAPIEndpoint)
	}

	// Updating the intervals must persist too.
	got.PWSUploadInterval = 300
	got.APRSUploadInterval = 1200
	got.AerisRefreshInterval = 0
	if err := p.UpdateDevice("svc-station", got); err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}
	after, err := p.GetDevice("svc-station")
	if err != nil {
		t.Fatalf("GetDevice after update: %v", err)
	}
	if after.PWSUploadInterval != 300 {
		t.Errorf("after update pws_upload_interval = %d, want 300", after.PWSUploadInterval)
	}
	if after.APRSUploadInterval != 1200 {
		t.Errorf("after update aprs_upload_interval = %d, want 1200", after.APRSUploadInterval)
	}
	if after.AerisRefreshInterval != 0 {
		t.Errorf("after update aeris_refresh_interval = %d, want 0", after.AerisRefreshInterval)
	}
}
