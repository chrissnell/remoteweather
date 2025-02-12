package main

import "math"

func calcWindChill(temp float32, windspeed float32) float32 {
	// For wind speeds < 3 or temps > 50, wind chill is just the current temperature
	if (temp > 50) || (windspeed < 3) {
		return temp
	}

	w64 := float64(windspeed)
	return (35.74 + (0.6215 * temp) - (35.75 * float32(math.Pow(w64, 0.16))) + (0.4275 * temp * float32(math.Pow(w64, 0.16))))
}

func calcHeatIndex(temp float32, humidity float32) float32 {

	// Heat indices don't make much sense at temps below 77° F, so just return the current temperature
	if temp < 77 {
		return temp
	}

	// First, we try Steadman's method, which is valid for all heat indices
	// below 80° F
	hi := 0.5 * (temp + 61.0 + ((temp - 68.0) * 1.2) + (humidity + 0.094))
	if hi < 80 {
		// Only return heat index if it's greater than the temperature
		if hi > temp {
			return hi
		}
		return temp
	}

	// Our heat index is > 80, so we need to use the Rothfusz method instead
	c1 := -42.379
	c2 := 2.04901523
	c3 := 10.14333127
	c4 := 0.22475541
	c5 := 0.00683783
	c6 := 0.05481717
	c7 := 0.00122874
	c8 := 0.00085282
	c9 := 0.00000199

	t64 := float64(temp)
	h64 := float64(humidity)

	hi64 := c1 + (c2 * t64) + (c3 * h64) - (c4 * t64 * h64) - (c5 * math.Pow(t64, 2)) - (c6 * math.Pow(h64, 2)) + (c7 * math.Pow(t64, 2) * h64) + (c8 * t64 * math.Pow(h64, 2)) - (c9 * math.Pow(t64, 2) * math.Pow(h64, 2))

	// If RH < 13% and temperature is between 80 and 112, we need to subtract an adjustment
	if humidity < 13 && temp >= 80 && temp <= 112 {
		adj := ((13 - h64) / 4) * math.Sqrt((17-math.Abs(t64-95.0))/17)
		hi64 = hi64 - adj
	} else if humidity > 80 && temp >= 80 && temp <= 87 {
		// Likewise, if RH > 80% and temperature is between 80 and 87, we need to add an adjustment
		adj := ((h64 - 85.0) / 10) * ((87.0 - t64) / 5)
		hi64 = hi64 + adj
	}

	// Only return heat index if it's greater than the temperature
	if hi64 > t64 {
		return float32(hi64)
	}
	return temp
}
