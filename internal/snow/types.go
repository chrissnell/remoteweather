package snow

import "time"

// SnowReading represents a single hourly snow depth reading
type SnowReading struct {
	Time    time.Time
	DepthMM float64
}

// Segment represents a classified time period
type Segment struct {
	StartTime      time.Time
	EndTime        time.Time
	StartDepthMM   float64
	EndDepthMM     float64
	MaxDepthMM     float64
	MinDepthMM     float64
	DurationHours  int
	NetChangeMM    float64
	MaxIncreaseMM  float64
	MaxDecreaseMM  float64
	Type           string  // "accumulation", "plateau", "redistribution", "spike_then_settle"
	SnowMM         float64 // Calculated snow accumulation for this segment
}
