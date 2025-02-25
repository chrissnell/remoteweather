package solar

import (
	"math"
	"time"
)

var timezoneMeridians = map[int]float64{
	-12: 180.0, -11: 165.0, -10: 150.0, -9: 135.0, -8: 120.0, -7: 105.0, -6: 90.0, -5: 75.0,
	-4: 60.0, -3: 45.0, -2: 30.0, -1: 15.0, 0: 0.0, 1: 15.0, 2: 30.0, 3: 45.0,
	4: 60.0, 5: 75.0, 6: 90.0, 7: 105.0, 8: 120.0, 9: 135.0, 10: 150.0, 11: 165.0,
	12: 180.0, 13: 195.0, 14: 210.0,
}

// CalculateClearSkySolarRadiationASCE computes clear-sky shortwave solar radiation (W/m²) using the ASCE model.
// Matches the JavaScript calculateModel function exactly.
func CalculateClearSkySolarRadiationASCE(t time.Time, latitude, longitude, altitude, airTempF, humidity float64) float64 {
	const solarConstant = 1361.0 // Solar constant (W/m²)
	const Kt = 1.0               // Clearness index

	// Day of year (1-365/366)
	dayOfYear := float64(t.UTC().YearDay())
	// Local time in decimal hours
	timeOfDay := float64(t.Hour()) + float64(t.Minute())/60.0 + float64(t.Second())/3600.0
	// Convert air temperature to °C
	airTemp := (airTempF - 32) * 5 / 9

	// Get local timezone offset in seconds
	_, offsetSecs := t.In(time.Local).Zone()
	// Lookup meridian longitude (positive west)
	lon_tz := timezoneMeridians[offsetSecs/3600]

	// Local longitude (positive west)
	lon := -longitude

	// Earth-Sun distance correction
	d_r := 1 + 0.033*math.Cos(((2*math.Pi)/365)*dayOfYear)

	// Solar declination (degrees)
	delta := (math.Asin(0.39785*math.Sin((278.97+0.9856*dayOfYear+1.9165*math.Sin((356.6+0.9856*dayOfYear)*(math.Pi/180)))*(math.Pi/180))) * 180) / math.Pi

	// Equation of time (hours)
	eqt := equationOfTime(t) / 60

	// Solar noon (local time, hours)
	solar_N := 12.0 + 0.0 - eqt - (lon_tz-lon)/15.0

	// Solar zenith angle (degrees)
	solar_Z := math.Acos(math.Sin(latitude*(math.Pi/180))*math.Sin(delta*(math.Pi/180))+math.Cos(latitude*(math.Pi/180))*math.Cos(delta*(math.Pi/180))*math.Cos((timeOfDay-solar_N)*(math.Pi/12))) * (180 / math.Pi)

	// Atmospheric pressure (kPa)
	P_B := 101.325 * math.Exp((altitude*-1*9.80665)/((8.314472/0.028967)*(airTemp+273.15)))

	// Vapor pressure (kPa)
	e_A := 0.61121 * math.Exp(((18.678-airTemp/234.5)*airTemp)/(257.14+airTemp)) * (humidity / 100)

	// Extraterrestrial radiation (W/m²)
	SW_a := solarConstant * d_r * math.Cos(solar_Z*(math.Pi/180))

	// Sun below horizon, no radiation
	if SW_a < 0 {
		return 0.0
	}

	// Precipitable water (mm)
	w := 0.15*e_A*P_B + 0.6

	// Beam transmittance
	K_b := 0.98 * math.Exp((-0.00146*P_B)/(Kt*math.Sin((90-solar_Z)*(math.Pi/180)))-0.075*math.Pow(w/math.Sin((90-solar_Z)*(math.Pi/180)), 0.4))

	// Diffuse transmittance
	var K_d float64

	if K_b > 0.15 {
		K_d = 0.35 - 0.36*K_b
	} else {
		K_d = 0.18 + 0.82*K_b
	}

	// Total shortwave radiation (W/m²)
	return (K_b + K_d) * SW_a
}
