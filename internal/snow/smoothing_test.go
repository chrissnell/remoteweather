package snow

import (
	"math"
	"testing"
	"time"
)

func TestLocalQuantileSmooth(t *testing.T) {
	tests := []struct {
		name     string
		samples  []Sample
		params   SmoothingParams
		expected []float64
		epsilon  float64
	}{
		{
			name: "single point",
			samples: []Sample{
				{Time: time.Now(), DepthIn: 10.0},
			},
			params: SmoothingParams{
				WindowMinutes: 30,
				Quantile:      0.85,
			},
			expected: []float64{10.0},
			epsilon:  0.01,
		},
		{
			name: "constant values",
			samples: []Sample{
				{Time: time.Now().Add(-2 * time.Hour), DepthIn: 5.0},
				{Time: time.Now().Add(-1 * time.Hour), DepthIn: 5.0},
				{Time: time.Now(), DepthIn: 5.0},
			},
			params: SmoothingParams{
				WindowMinutes: 120, // 2-hour window
				Quantile:      0.85,
			},
			expected: []float64{5.0, 5.0, 5.0},
			epsilon:  0.01,
		},
		{
			name: "upper quantile filters noise",
			samples: []Sample{
				{Time: time.Now().Add(-40 * time.Minute), DepthIn: 10.0},
				{Time: time.Now().Add(-30 * time.Minute), DepthIn: 2.0}, // Noise (low outlier)
				{Time: time.Now().Add(-20 * time.Minute), DepthIn: 10.0},
				{Time: time.Now().Add(-10 * time.Minute), DepthIn: 10.0},
				{Time: time.Now(), DepthIn: 10.0},
			},
			params: SmoothingParams{
				WindowMinutes: 30, // ±30 min window
				Quantile:      0.85,
			},
			// The middle points (indices 2,3,4) should be close to 10
			// Edge points may show some influence from the outlier
			expected: []float64{8.0, 10.0, 10.0, 10.0, 10.0},
			epsilon:  2.0, // Allow some variation at edges
		},
		{
			name: "accumulation event preserved",
			samples: []Sample{
				{Time: time.Now().Add(-3 * time.Hour), DepthIn: 5.0},
				{Time: time.Now().Add(-2 * time.Hour), DepthIn: 8.0},
				{Time: time.Now().Add(-1 * time.Hour), DepthIn: 11.0},
				{Time: time.Now(), DepthIn: 14.0},
			},
			params: SmoothingParams{
				WindowMinutes: 90, // 1.5-hour window
				Quantile:      0.85,
			},
			// Should preserve upward trend
			expected: []float64{5.0, 8.0, 11.0, 14.0},
			epsilon:  2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LocalQuantileSmooth(tt.samples, tt.params)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d results, got %d", len(tt.expected), len(result))
			}

			for i, val := range result {
				if math.Abs(val-tt.expected[i]) > tt.epsilon {
					t.Errorf("point %d: expected %.2f ± %.2f, got %.2f",
						i, tt.expected[i], tt.epsilon, val)
				}
			}
		})
	}
}

func TestApplyRateLimiting(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		samples        []Sample
		smoothed       []float64
		params         SmoothingParams
		prevEstimate   *Sample
		expected       []float64
		epsilon        float64
		checkSpecific  bool
		specificChecks map[int]float64
	}{
		{
			name: "no rate limiting needed",
			samples: []Sample{
				{Time: now.Add(-2 * time.Hour), DepthIn: 5.0},
				{Time: now.Add(-1 * time.Hour), DepthIn: 6.0},
				{Time: now, DepthIn: 7.0},
			},
			smoothed: []float64{5.0, 6.0, 7.0},
			params: SmoothingParams{
				MaxUpRateInPerHour:   4.0,
				MaxDownRateInPerHour: 1.5,
			},
			expected: []float64{5.0, 6.0, 7.0},
			epsilon:  0.01,
		},
		{
			name: "cap excessive accumulation",
			samples: []Sample{
				{Time: now.Add(-1 * time.Hour), DepthIn: 5.0},
				{Time: now, DepthIn: 15.0}, // 10 inches in 1 hour - too fast!
			},
			smoothed: []float64{5.0, 15.0},
			params: SmoothingParams{
				MaxUpRateInPerHour:   4.0, // Max 4 inches/hour
				MaxDownRateInPerHour: 1.5,
			},
			expected: []float64{5.0, 9.0}, // 5.0 + (4.0 * 1 hour) = 9.0
			epsilon:  0.01,
		},
		{
			name: "cap excessive settling",
			samples: []Sample{
				{Time: now.Add(-1 * time.Hour), DepthIn: 15.0},
				{Time: now, DepthIn: 5.0}, // 10 inches decrease in 1 hour - too fast!
			},
			smoothed: []float64{15.0, 5.0},
			params: SmoothingParams{
				MaxUpRateInPerHour:   4.0,
				MaxDownRateInPerHour: 1.5, // Max 1.5 inches/hour decrease
			},
			expected: []float64{15.0, 13.5}, // 15.0 - (1.5 * 1 hour) = 13.5
			epsilon:  0.01,
		},
		{
			name: "with previous estimate",
			samples: []Sample{
				{Time: now, DepthIn: 20.0}, // Big jump from previous estimate
			},
			smoothed: []float64{20.0},
			params: SmoothingParams{
				MaxUpRateInPerHour:   4.0,
				MaxDownRateInPerHour: 1.5,
			},
			prevEstimate: &Sample{
				Time:    now.Add(-1 * time.Hour),
				DepthIn: 10.0,
			},
			expected: []float64{14.0}, // 10.0 + (4.0 * 1 hour) = 14.0
			epsilon:  0.01,
		},
		{
			name: "multiple points with rate limiting",
			samples: []Sample{
				{Time: now.Add(-3 * time.Hour), DepthIn: 5.0},
				{Time: now.Add(-2 * time.Hour), DepthIn: 15.0}, // Too fast
				{Time: now.Add(-1 * time.Hour), DepthIn: 20.0}, // Still too fast
				{Time: now, DepthIn: 25.0},                     // Still too fast
			},
			smoothed: []float64{5.0, 15.0, 20.0, 25.0},
			params: SmoothingParams{
				MaxUpRateInPerHour:   4.0,
				MaxDownRateInPerHour: 1.5,
			},
			checkSpecific: true,
			specificChecks: map[int]float64{
				0: 5.0,  // First value unchanged
				1: 9.0,  // 5.0 + 4.0*1 = 9.0
				2: 13.0, // 9.0 + 4.0*1 = 13.0
				3: 17.0, // 13.0 + 4.0*1 = 17.0
			},
			epsilon: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyRateLimiting(tt.samples, tt.smoothed, tt.params, tt.prevEstimate)

			if len(result) != len(tt.expected) && !tt.checkSpecific {
				t.Fatalf("expected %d results, got %d", len(tt.expected), len(result))
			}

			if tt.checkSpecific {
				for idx, expectedVal := range tt.specificChecks {
					if idx >= len(result) {
						t.Errorf("index %d out of range (result length %d)", idx, len(result))
						continue
					}
					if math.Abs(result[idx]-expectedVal) > tt.epsilon {
						t.Errorf("point %d: expected %.2f ± %.2f, got %.2f",
							idx, expectedVal, tt.epsilon, result[idx])
					}
				}
			} else {
				for i, val := range result {
					if math.Abs(val-tt.expected[i]) > tt.epsilon {
						t.Errorf("point %d: expected %.2f ± %.2f, got %.2f",
							i, tt.expected[i], tt.epsilon, val)
					}
				}
			}
		})
	}
}

func TestGetSeasonStart(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "in winter (January)",
			now:      time.Date(2024, time.January, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "in summer (July)",
			now:      time.Date(2024, time.July, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "in fall before October",
			now:      time.Date(2024, time.September, 30, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "on October 1",
			now:      time.Date(2024, time.October, 1, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "in fall after October",
			now:      time.Date(2024, time.November, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "in winter (December)",
			now:      time.Date(2024, time.December, 31, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSeasonStart(tt.now)
			if !result.Equal(tt.expected) {
				t.Errorf("expected %s, got %s", tt.expected.Format("2006-01-02"), result.Format("2006-01-02"))
			}
		})
	}
}

func TestDefaultSmoothingParams(t *testing.T) {
	params := DefaultSmoothingParams()

	if params.WindowMinutes <= 0 {
		t.Errorf("WindowMinutes should be positive, got %d", params.WindowMinutes)
	}

	if params.Quantile <= 0 || params.Quantile >= 1 {
		t.Errorf("Quantile should be between 0 and 1, got %.2f", params.Quantile)
	}

	if params.MaxUpRateInPerHour <= 0 {
		t.Errorf("MaxUpRateInPerHour should be positive, got %.2f", params.MaxUpRateInPerHour)
	}

	if params.MaxDownRateInPerHour <= 0 {
		t.Errorf("MaxDownRateInPerHour should be positive, got %.2f", params.MaxDownRateInPerHour)
	}

	// Verify reasonable default values
	if params.WindowMinutes != 30 {
		t.Errorf("expected default WindowMinutes=30, got %d", params.WindowMinutes)
	}

	if params.Quantile != 0.85 {
		t.Errorf("expected default Quantile=0.85, got %.2f", params.Quantile)
	}

	if params.MaxUpRateInPerHour != 4.0 {
		t.Errorf("expected default MaxUpRateInPerHour=4.0, got %.2f", params.MaxUpRateInPerHour)
	}

	if params.MaxDownRateInPerHour != 1.5 {
		t.Errorf("expected default MaxDownRateInPerHour=1.5, got %.2f", params.MaxDownRateInPerHour)
	}
}
