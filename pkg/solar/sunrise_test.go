package solar

import (
	"math"
	"testing"
	"time"
)

func TestCalculateSunriseSunset(t *testing.T) {
	tests := []struct {
		name             string
		dayOfYear        int
		latitude         float64
		longitude        float64
		expectSunrise    bool // false if polar conditions
		sunriseApproxUTC int  // approximate expected sunrise in UTC minutes (±60 min tolerance)
		sunsetApproxUTC  int  // approximate expected sunset in UTC minutes (±60 min tolerance)
	}{
		{
			name:             "Equator at equinox (March 20, day 79)",
			dayOfYear:        79,
			latitude:         0.0,
			longitude:        0.0,
			expectSunrise:    true,
			sunriseApproxUTC: 360, // ~6:00 AM UTC
			sunsetApproxUTC:  1080, // ~6:00 PM UTC
		},
		{
			name:             "Seattle WA summer solstice (June 21, day 172)",
			dayOfYear:        172,
			latitude:         47.6,
			longitude:        -122.3,
			expectSunrise:    true,
			sunriseApproxUTC: 730, // ~12:10 PM UTC (5:10 AM PDT)
			sunsetApproxUTC:  250, // ~4:10 AM UTC next day (9:10 PM PDT, wraps at midnight)
		},
		{
			name:             "Seattle WA winter solstice (Dec 21, day 355)",
			dayOfYear:        355,
			latitude:         47.6,
			longitude:        -122.3,
			expectSunrise:    true,
			sunriseApproxUTC: 960, // ~4:00 PM UTC (8:00 AM PST)
			sunsetApproxUTC:  10,  // ~12:10 AM UTC next day (4:10 PM PST, wraps at midnight)
		},
		{
			name:             "London UK summer",
			dayOfYear:        172,
			latitude:         51.5,
			longitude:        -0.1,
			expectSunrise:    true,
			sunriseApproxUTC: 260, // ~4:20 AM UTC
			sunsetApproxUTC:  1260, // ~9:00 PM UTC
		},
		{
			name:             "Arctic circle summer (polar day)",
			dayOfYear:        172,
			latitude:         70.0,
			longitude:        25.0,
			expectSunrise:    false, // sun doesn't set
			sunriseApproxUTC: -1,
			sunsetApproxUTC:  -1,
		},
		{
			name:             "Arctic circle winter (polar night)",
			dayOfYear:        355,
			latitude:         70.0,
			longitude:        25.0,
			expectSunrise:    false, // sun doesn't rise
			sunriseApproxUTC: -1,
			sunsetApproxUTC:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sunrise, sunset, err := CalculateSunriseSunset(tt.dayOfYear, tt.latitude, tt.longitude)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectSunrise {
				if sunrise < 0 || sunset < 0 {
					t.Errorf("expected valid sunrise/sunset, got sunrise=%d, sunset=%d", sunrise, sunset)
					return
				}

				// Check sunrise within tolerance (±60 min to account for algorithm variations)
				tolerance := 60
				if diff := int(math.Abs(float64(sunrise - tt.sunriseApproxUTC))); diff > tolerance && diff < 1440-tolerance {
					t.Errorf("sunrise=%d minutes, expected ~%d minutes (±%d)", sunrise, tt.sunriseApproxUTC, tolerance)
				}

				if diff := int(math.Abs(float64(sunset - tt.sunsetApproxUTC))); diff > tolerance && diff < 1440-tolerance {
					t.Errorf("sunset=%d minutes, expected ~%d minutes (±%d)", sunset, tt.sunsetApproxUTC, tolerance)
				}
			} else {
				if sunrise != -1 || sunset != -1 {
					t.Errorf("expected polar conditions (sunrise=-1, sunset=-1), got sunrise=%d, sunset=%d", sunrise, sunset)
				}
			}
		})
	}
}

func TestFormatSunTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name       string
		utcMinutes int
		loc        *time.Location
		expected   string
	}{
		{
			name:       "Morning UTC to Pacific (winter/PST)",
			utcMinutes: 840, // 2:00 PM UTC
			loc:        loc,
			expected:   "6:00 AM", // FormatSunTime uses Jan 1 (PST, UTC-8)
		},
		{
			name:       "Negative minutes returns empty",
			utcMinutes: -1,
			loc:        loc,
			expected:   "",
		},
		{
			name:       "Noon UTC",
			utcMinutes: 720, // 12:00 PM UTC
			loc:        time.UTC,
			expected:   "12:00 PM",
		},
		{
			name:       "Midnight UTC",
			utcMinutes: 0,
			loc:        time.UTC,
			expected:   "12:00 AM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSunTime(tt.utcMinutes, tt.loc)
			if result != tt.expected {
				t.Errorf("FormatSunTime(%d) = %q, expected %q", tt.utcMinutes, result, tt.expected)
			}
		})
	}
}

func TestSunriseSunsetConsistency(t *testing.T) {
	// Test that sunrise is always before sunset for mid-latitudes
	for doy := 1; doy <= 365; doy++ {
		sunrise, sunset, err := CalculateSunriseSunset(doy, 45.0, 0.0) // Mid-latitude, prime meridian
		if err != nil {
			t.Fatalf("day %d: unexpected error: %v", doy, err)
		}

		if sunrise < 0 || sunset < 0 {
			t.Errorf("day %d: unexpected polar conditions at 45°N", doy)
			continue
		}

		// Day length should be reasonable (4-20 hours at 45° latitude)
		var dayLength int
		if sunset > sunrise {
			dayLength = sunset - sunrise
		} else {
			dayLength = (1440 - sunrise) + sunset // crosses midnight
		}

		if dayLength < 240 || dayLength > 1200 { // 4-20 hours
			t.Errorf("day %d: unreasonable day length: %d minutes", doy, dayLength)
		}
	}
}
