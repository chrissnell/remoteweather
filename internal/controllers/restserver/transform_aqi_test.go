package restserver

import (
	"testing"

	"github.com/chrissnell/remoteweather/internal/types"
	"github.com/chrissnell/remoteweather/pkg/aqi"
)

func TestGetOrCalculateAQIPM25(t *testing.T) {
	// Stored AQI is preferred when present.
	if got := getOrCalculateAQIPM25(types.Reading{AQIPM25AQIN: 142, PM25: 5}); got != 142 {
		t.Errorf("stored AQI: got %d, want 142", got)
	}
	// Otherwise computed from PM2.5 via pkg/aqi.
	if got, want := getOrCalculateAQIPM25(types.Reading{PM25: 35.4}), aqi.CalculatePM25(35.4); got != want {
		t.Errorf("computed AQI: got %d, want %d", got, want)
	}
	// No data -> 0.
	if got := getOrCalculateAQIPM25(types.Reading{}); got != 0 {
		t.Errorf("no data: got %d, want 0", got)
	}
}

func TestGetOrCalculateAQIPM10(t *testing.T) {
	// Stored AQI is preferred when present.
	if got := getOrCalculateAQIPM10(types.Reading{AQIPM10AQIN: 75, PM10InAQIN: 10}); got != 75 {
		t.Errorf("stored AQI: got %d, want 75", got)
	}
	// Regression guard: previously stubbed to 0; must now compute from PM10.
	if got, want := getOrCalculateAQIPM10(types.Reading{PM10InAQIN: 154}), aqi.CalculatePM10(154); got != want {
		t.Errorf("computed AQI: got %d, want %d", got, want)
	}
	if got := getOrCalculateAQIPM10(types.Reading{PM10InAQIN: 154}); got == 0 {
		t.Error("computed PM10 AQI is 0; the stub was not fixed")
	}
	// No data -> 0.
	if got := getOrCalculateAQIPM10(types.Reading{}); got != 0 {
		t.Errorf("no data: got %d, want 0", got)
	}
}
