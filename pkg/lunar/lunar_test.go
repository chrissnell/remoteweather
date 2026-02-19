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

func TestCalculateCrescentAngle(t *testing.T) {
	t.Run("first quarter northern hemisphere", func(t *testing.T) {
		// Jan 28, 2023 15:19 UTC — first quarter
		// Observer in New York (40.7°N, 74.0°W)
		ts := time.Date(2023, 1, 28, 15, 19, 0, 0, time.UTC)
		result := CalculateCrescentAngle(ts, 40.7, -74.0)

		// Bright limb should point roughly toward the Sun (westward in afternoon)
		if math.IsNaN(result.Rotation) {
			t.Fatal("Rotation is NaN")
		}

		// Illumination should be ~0.5 at first quarter
		if result.Illumination < 0.4 || result.Illumination > 0.6 {
			t.Errorf("Illumination = %.3f, expected ~0.5 at first quarter", result.Illumination)
		}

		// Phase angle should be ~90° at quarter
		if result.PhaseAngle < 70 || result.PhaseAngle > 110 {
			t.Errorf("PhaseAngle = %.1f°, expected ~90° at first quarter", result.PhaseAngle)
		}

		t.Logf("First quarter NYC: rotation=%.1f° chi=%.1f° theta=%.1f° q=%.1f° theta_local=%.1f° k=%.3f",
			result.Rotation, result.BrightLimbAngle, result.TerminatorAngle,
			result.ParallacticAngle, result.LocalTerminator, result.Illumination)
	})

	t.Run("no location fallback", func(t *testing.T) {
		ts := time.Date(2023, 1, 28, 15, 19, 0, 0, time.UTC)
		result := CalculateCrescentAngle(ts, 0, 0)

		if math.IsNaN(result.Rotation) {
			t.Fatal("Rotation is NaN with zero lat/lon")
		}
		// Parallactic angle should be zero (skipped)
		if result.ParallacticAngle != 0 {
			t.Errorf("ParallacticAngle = %.1f, expected 0 for zero lat/lon", result.ParallacticAngle)
		}
	})

	t.Run("southern hemisphere differs from northern", func(t *testing.T) {
		ts := time.Date(2023, 1, 28, 20, 0, 0, 0, time.UTC)
		north := CalculateCrescentAngle(ts, 40.7, -74.0)  // NYC
		south := CalculateCrescentAngle(ts, -33.9, 18.4)  // Cape Town

		diff := math.Abs(north.Rotation - south.Rotation)
		if diff < 10 {
			t.Errorf("N/S rotation difference = %.1f°, expected significant difference (>10°)", diff)
		}
		t.Logf("Northern rotation=%.1f°, Southern rotation=%.1f°, diff=%.1f°",
			north.Rotation, south.Rotation, diff)
	})

	t.Run("full moon rotation is computed without error", func(t *testing.T) {
		// Feb 5, 2023 18:29 UTC — full moon
		ts := time.Date(2023, 2, 5, 18, 29, 0, 0, time.UTC)
		result := CalculateCrescentAngle(ts, 40.7, -74.0)

		if math.IsNaN(result.Rotation) {
			t.Fatal("Rotation is NaN at full moon")
		}
		// Illumination should be ~1.0
		if result.Illumination < 0.95 {
			t.Errorf("Illumination = %.3f, expected >0.95 at full moon", result.Illumination)
		}
	})

	t.Run("new moon rotation is computed without error", func(t *testing.T) {
		// Jan 21, 2023 20:53 UTC — new moon
		ts := time.Date(2023, 1, 21, 20, 53, 0, 0, time.UTC)
		result := CalculateCrescentAngle(ts, 40.7, -74.0)

		if math.IsNaN(result.Rotation) {
			t.Fatal("Rotation is NaN at new moon")
		}
		// Illumination should be ~0.0
		if result.Illumination > 0.05 {
			t.Errorf("Illumination = %.3f, expected <0.05 at new moon", result.Illumination)
		}
	})

	t.Run("angle range check across a year", func(t *testing.T) {
		// Verify no NaN or infinity over a full year at various locations
		locations := [][2]float64{
			{40.7, -74.0},   // NYC
			{-33.9, 18.4},   // Cape Town
			{51.5, -0.1},    // London
			{35.7, 139.7},   // Tokyo
			{-23.5, -46.6},  // São Paulo
			{64.1, -21.9},   // Reykjavik
		}
		for _, loc := range locations {
			for month := 1; month <= 12; month++ {
				ts := time.Date(2023, time.Month(month), 15, 22, 0, 0, 0, time.UTC)
				result := CalculateCrescentAngle(ts, loc[0], loc[1])
				if math.IsNaN(result.Rotation) || math.IsInf(result.Rotation, 0) {
					t.Errorf("Bad rotation at lat=%.1f lon=%.1f month=%d: %f",
						loc[0], loc[1], month, result.Rotation)
				}
			}
		}
	})
}

func TestEquatorialConversion(t *testing.T) {
	// Verify ecliptic-to-equatorial conversion at vernal equinox point
	// λ=0, β=0 should give RA=0, Dec=0 regardless of obliquity
	ra, dec := eclipticToEquatorial(0, 0, 23.44)
	if math.Abs(ra) > 0.001 || math.Abs(dec) > 0.001 {
		t.Errorf("Vernal equinox: RA=%.6f Dec=%.6f, expected ~0", ra, dec)
	}

	// Summer solstice point λ=90°, β=0 → RA=6h (π/2), Dec=ε
	ra, dec = eclipticToEquatorial(90, 0, 23.44)
	expectedRA := math.Pi / 2
	expectedDec := degToRad(23.44)
	if math.Abs(ra-expectedRA) > 0.01 || math.Abs(dec-expectedDec) > 0.01 {
		t.Errorf("Summer solstice: RA=%.4f (exp %.4f) Dec=%.4f (exp %.4f)",
			ra, expectedRA, dec, expectedDec)
	}
}

func TestGreenwichSiderealTime(t *testing.T) {
	// J2000.0 epoch: GMST should be approximately 18h 41m = 280.46°
	jd := 2451545.0 // J2000.0
	gmst := greenwichMeanSiderealTime(jd)
	// Expected ~280.46° (18.6972h * 15)
	if math.Abs(gmst-280.46) > 1.0 {
		t.Errorf("GMST at J2000.0 = %.2f°, expected ~280.46°", gmst)
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

func BenchmarkCalculate(b *testing.B) {
	ts := time.Date(2023, 1, 28, 15, 19, 0, 0, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(ts)
	}
}

func BenchmarkCalculateCrescentAngle(b *testing.B) {
	ts := time.Date(2023, 1, 28, 15, 19, 0, 0, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateCrescentAngle(ts, 40.7, -74.0)
	}
}
