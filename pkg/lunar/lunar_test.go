package lunar

import (
	"math"
	"testing"
	"time"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name              string
		time              time.Time
		expectedPhaseName string
		illuminationRange [2]float64 // min, max
		isWaxing          bool
	}{
		{
			// Known new moon: Jan 21, 2023 20:53 UTC
			name:              "New Moon Jan 2023",
			time:              time.Date(2023, 1, 21, 20, 53, 0, 0, time.UTC),
			expectedPhaseName: "New Moon",
			illuminationRange: [2]float64{0.0, 0.05},
			isWaxing:          true,
		},
		{
			// Known full moon: Feb 5, 2023 18:29 UTC
			name:              "Full Moon Feb 2023",
			time:              time.Date(2023, 2, 5, 18, 29, 0, 0, time.UTC),
			expectedPhaseName: "Full Moon",
			illuminationRange: [2]float64{0.95, 1.0},
			isWaxing:          false,
		},
		{
			// Known first quarter: Jan 28, 2023 15:19 UTC
			name:              "First Quarter Jan 2023",
			time:              time.Date(2023, 1, 28, 15, 19, 0, 0, time.UTC),
			expectedPhaseName: "First Quarter",
			illuminationRange: [2]float64{0.45, 0.55},
			isWaxing:          true,
		},
		{
			// Known third quarter: Feb 13, 2023 16:01 UTC
			name:              "Third Quarter Feb 2023",
			time:              time.Date(2023, 2, 13, 16, 1, 0, 0, time.UTC),
			expectedPhaseName: "Third Quarter",
			illuminationRange: [2]float64{0.45, 0.55},
			isWaxing:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Calculate(tt.time)

			if result.PhaseName != tt.expectedPhaseName {
				t.Errorf("PhaseName = %q, expected %q", result.PhaseName, tt.expectedPhaseName)
			}

			if result.Illumination < tt.illuminationRange[0] || result.Illumination > tt.illuminationRange[1] {
				t.Errorf("Illumination = %.3f, expected in range [%.2f, %.2f]",
					result.Illumination, tt.illuminationRange[0], tt.illuminationRange[1])
			}

			if result.IsWaxing != tt.isWaxing {
				t.Errorf("IsWaxing = %v, expected %v", result.IsWaxing, tt.isWaxing)
			}
		})
	}
}

func TestPhaseProgression(t *testing.T) {
	// Test that phase increases monotonically over a lunar cycle
	start := time.Date(2023, 1, 21, 20, 53, 0, 0, time.UTC) // New moon
	prevPhase := -1.0

	for day := 0; day < 29; day++ {
		currentTime := start.Add(time.Duration(day) * 24 * time.Hour)
		result := Calculate(currentTime)

		// Phase should generally increase (allowing for wrap-around near 1.0)
		if prevPhase >= 0 && prevPhase < 0.9 {
			if result.Phase < prevPhase-0.01 {
				t.Errorf("Day %d: phase decreased from %.3f to %.3f", day, prevPhase, result.Phase)
			}
		}
		prevPhase = result.Phase
	}
}

func TestIlluminationRange(t *testing.T) {
	// Test illumination stays in valid range [0, 1] for random times
	for year := 2020; year <= 2025; year++ {
		for month := 1; month <= 12; month++ {
			testTime := time.Date(year, time.Month(month), 15, 12, 0, 0, 0, time.UTC)
			result := Calculate(testTime)

			if result.Illumination < 0 || result.Illumination > 1 {
				t.Errorf("Illumination %.3f out of range [0, 1] for %v", result.Illumination, testTime)
			}

			if result.Phase < 0 || result.Phase >= 1 {
				t.Errorf("Phase %.3f out of range [0, 1) for %v", result.Phase, testTime)
			}

			if result.AgeDays < 0 || result.AgeDays >= SynodicMonth {
				t.Errorf("AgeDays %.3f out of range [0, %.3f) for %v", result.AgeDays, SynodicMonth, testTime)
			}

			if result.Elongation < 0 || result.Elongation >= 360 {
				t.Errorf("Elongation %.3f out of range [0, 360) for %v", result.Elongation, testTime)
			}
		}
	}
}

func TestPhaseNameCoverage(t *testing.T) {
	// Test that all 8 phase names are produced over a lunar cycle
	// Sample every 3 hours to catch narrow quarter windows (49-51% illumination)
	start := time.Date(2023, 1, 21, 20, 53, 0, 0, time.UTC)
	phaseNames := make(map[string]bool)

	for hour := 0; hour < 30*24; hour += 3 {
		currentTime := start.Add(time.Duration(hour) * time.Hour)
		result := Calculate(currentTime)
		phaseNames[result.PhaseName] = true
	}

	expectedPhases := []string{
		"New Moon", "Waxing Crescent", "First Quarter", "Waxing Gibbous",
		"Full Moon", "Waning Gibbous", "Third Quarter", "Waning Crescent",
	}

	for _, phase := range expectedPhases {
		if !phaseNames[phase] {
			t.Errorf("Phase %q not observed over lunar cycle", phase)
		}
	}
}

func TestSynodicMonth(t *testing.T) {
	// Verify synodic month constant matches expected value
	expected := 29.530588853
	if math.Abs(SynodicMonth-expected) > 0.000001 {
		t.Errorf("SynodicMonth = %.9f, expected %.9f", SynodicMonth, expected)
	}
}

func TestWaxingWaning(t *testing.T) {
	// New moon to full moon should be waxing
	newMoon := time.Date(2023, 1, 21, 20, 53, 0, 0, time.UTC)
	result := Calculate(newMoon.Add(7 * 24 * time.Hour))
	if !result.IsWaxing {
		t.Error("Expected waxing phase 7 days after new moon")
	}

	// Full moon to new moon should be waning
	fullMoon := time.Date(2023, 2, 5, 18, 29, 0, 0, time.UTC)
	result = Calculate(fullMoon.Add(7 * 24 * time.Hour))
	if result.IsWaxing {
		t.Error("Expected waning phase 7 days after full moon")
	}
}
