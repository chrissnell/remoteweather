package campbell

// CalculateWindChill calculates wind chill temperature (simplified)
func CalculateWindChill(tempF, windSpeedMph float32) float32 {
	if tempF > 50 || windSpeedMph < 3 {
		return tempF // No wind chill above 50°F or below 3 mph
	}
	return 35.74 + 0.6215*tempF - 35.75*pow(windSpeedMph, 0.16) + 0.4275*tempF*pow(windSpeedMph, 0.16)
}

// CalculateHeatIndex calculates heat index (simplified)
func CalculateHeatIndex(tempF, humidity float32) float32 {
	if tempF < 80 {
		return tempF // No heat index below 80°F
	}
	c1 := float32(-42.379)
	c2 := float32(2.04901523)
	c3 := float32(10.14333127)
	c4 := float32(-0.22475541)
	c5 := float32(-0.00683783)
	c6 := float32(-0.05481717)
	c7 := float32(0.00122874)
	c8 := float32(0.00085282)
	c9 := float32(-0.00000199)

	hi := c1 + c2*tempF + c3*humidity + c4*tempF*humidity + c5*tempF*tempF +
		c6*humidity*humidity + c7*tempF*tempF*humidity + c8*tempF*humidity*humidity +
		c9*tempF*tempF*humidity*humidity

	return hi
}

// pow provides a simple power function for wind chill calculation
func pow(x, y float32) float32 {
	if y == 0.16 {
		// Special case for wind chill calculation
		// x^0.16 ≈ approximation for small values
		return 1.0 + 0.16*x + 0.0128*x*x // Taylor series approximation
	}
	return x // Fallback
}
