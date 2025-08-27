// Package aqi provides functions for calculating Air Quality Index values
// from particulate matter concentrations according to EPA standards
package aqi

import "math"

// CalculatePM25 calculates the Air Quality Index from PM2.5 concentration (μg/m³)
// Based on EPA AQI calculation formula for 24-hour PM2.5 averages
func CalculatePM25(pm25 float32) int32 {
	if pm25 < 0 {
		return 0
	}
	
	// EPA breakpoints for PM2.5
	var cLow, cHigh, iLow, iHigh float64
	pm := float64(pm25)
	
	switch {
	case pm <= 12.0:
		cLow, cHigh = 0.0, 12.0
		iLow, iHigh = 0, 50
	case pm <= 35.4:
		cLow, cHigh = 12.1, 35.4
		iLow, iHigh = 51, 100
	case pm <= 55.4:
		cLow, cHigh = 35.5, 55.4
		iLow, iHigh = 101, 150
	case pm <= 150.4:
		cLow, cHigh = 55.5, 150.4
		iLow, iHigh = 151, 200
	case pm <= 250.4:
		cLow, cHigh = 150.5, 250.4
		iLow, iHigh = 201, 300
	case pm <= 350.4:
		cLow, cHigh = 250.5, 350.4
		iLow, iHigh = 301, 400
	case pm <= 500.4:
		cLow, cHigh = 350.5, 500.4
		iLow, iHigh = 401, 500
	default:
		// Beyond 500.4, AQI is 500+
		return 500
	}
	
	// AQI calculation formula: I = (I_high - I_low) / (C_high - C_low) * (C - C_low) + I_low
	aqi := ((iHigh - iLow) / (cHigh - cLow)) * (pm - cLow) + iLow
	return int32(math.Round(aqi))
}

// CalculatePM10 calculates the Air Quality Index from PM10 concentration (μg/m³)
// Based on EPA AQI calculation formula for 24-hour PM10 averages
func CalculatePM10(pm10 float32) int32 {
	if pm10 < 0 {
		return 0
	}
	
	// EPA breakpoints for PM10
	var cLow, cHigh, iLow, iHigh float64
	pm := float64(pm10)
	
	switch {
	case pm <= 54:
		cLow, cHigh = 0, 54
		iLow, iHigh = 0, 50
	case pm <= 154:
		cLow, cHigh = 55, 154
		iLow, iHigh = 51, 100
	case pm <= 254:
		cLow, cHigh = 155, 254
		iLow, iHigh = 101, 150
	case pm <= 354:
		cLow, cHigh = 255, 354
		iLow, iHigh = 151, 200
	case pm <= 424:
		cLow, cHigh = 355, 424
		iLow, iHigh = 201, 300
	case pm <= 504:
		cLow, cHigh = 425, 504
		iLow, iHigh = 301, 400
	case pm <= 604:
		cLow, cHigh = 505, 604
		iLow, iHigh = 401, 500
	default:
		// Beyond 604, AQI is 500+
		return 500
	}
	
	// AQI calculation formula: I = (I_high - I_low) / (C_high - C_low) * (C - C_low) + I_low
	aqi := ((iHigh - iLow) / (cHigh - cLow)) * (pm - cLow) + iLow
	return int32(math.Round(aqi))
}

// GetCategory returns the AQI category name for a given AQI value
func GetCategory(aqi int32) string {
	switch {
	case aqi <= 50:
		return "Good"
	case aqi <= 100:
		return "Moderate"
	case aqi <= 150:
		return "Unhealthy for Sensitive Groups"
	case aqi <= 200:
		return "Unhealthy"
	case aqi <= 300:
		return "Very Unhealthy"
	default:
		return "Hazardous"
	}
}

// GetCategoryColor returns the standard color code for an AQI value
func GetCategoryColor(aqi int32) string {
	switch {
	case aqi <= 50:
		return "#00e400" // Green
	case aqi <= 100:
		return "#ffff00" // Yellow
	case aqi <= 150:
		return "#ff7e00" // Orange
	case aqi <= 200:
		return "#ff0000" // Red
	case aqi <= 300:
		return "#99004c" // Purple
	default:
		return "#7e0023" // Maroon
	}
}