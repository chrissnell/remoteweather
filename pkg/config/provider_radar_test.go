package config

import (
	"path/filepath"
	"testing"
)

func newTestProvider(t *testing.T) *SQLiteProvider {
	t.Helper()
	p, err := NewSQLiteProvider(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteProvider: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func TestWeatherWebsiteRadarDefaults(t *testing.T) {
	p := newTestProvider(t)
	w := &WeatherWebsiteData{Name: "Radar Test", Hostname: "radar.example.com", IsPortal: true}
	if err := p.AddWeatherWebsite(w); err != nil {
		t.Fatalf("AddWeatherWebsite: %v", err)
	}
	got, err := p.GetWeatherWebsite(w.ID)
	if err != nil {
		t.Fatalf("GetWeatherWebsite: %v", err)
	}
	if got.RadarEnabled {
		t.Errorf("RadarEnabled = true, want false (default)")
	}
	if got.RadarToken != "" {
		t.Errorf("RadarToken = %q, want empty", got.RadarToken)
	}
	if got.RadarRegisteredAt != nil {
		t.Errorf("RadarRegisteredAt = %v, want nil", got.RadarRegisteredAt)
	}
}

func TestWeatherWebsiteRadarRegistration(t *testing.T) {
	p := newTestProvider(t)
	w := &WeatherWebsiteData{Name: "Reg Test", Hostname: "reg.example.com", IsPortal: true}
	if err := p.AddWeatherWebsite(w); err != nil {
		t.Fatalf("AddWeatherWebsite: %v", err)
	}

	if err := p.SetWebsiteRadarRegistration(w.ID, "tok-123", 1700000000); err != nil {
		t.Fatalf("SetWebsiteRadarRegistration: %v", err)
	}
	got, _ := p.GetWeatherWebsite(w.ID)
	if !got.RadarEnabled || got.RadarToken != "tok-123" ||
		got.RadarRegisteredAt == nil || *got.RadarRegisteredAt != 1700000000 {
		t.Fatalf("after register: %+v", got)
	}

	// A generic form update must NOT clobber the radar registration.
	got.Name = "Renamed"
	if err := p.UpdateWeatherWebsite(w.ID, got); err != nil {
		t.Fatalf("UpdateWeatherWebsite: %v", err)
	}
	after, _ := p.GetWeatherWebsite(w.ID)
	if !after.RadarEnabled || after.RadarToken != "tok-123" {
		t.Fatalf("form update clobbered radar: %+v", after)
	}

	if err := p.ClearWebsiteRadarRegistration(w.ID); err != nil {
		t.Fatalf("ClearWebsiteRadarRegistration: %v", err)
	}
	cleared, _ := p.GetWeatherWebsite(w.ID)
	if cleared.RadarEnabled || cleared.RadarToken != "" || cleared.RadarRegisteredAt != nil {
		t.Fatalf("after clear: %+v", cleared)
	}
}
