// Package lunar provides moon phase calculations using ecliptic longitudes
// of the Sun and Moon. Accuracy is typically within ~0.5-1% illumination
// and ~1-2 hours of phase angle.
package lunar

import (
	"math"
	"time"
)

// SynodicMonth is the average length of the lunar cycle in days
const SynodicMonth = 29.530588853

// MoonPhase contains calculated moon phase information
type MoonPhase struct {
	Phase        float64 // Phase fraction [0,1): 0=new, 0.5=full
	Elongation   float64 // Sun→Moon angle in degrees [0,360)
	Illumination float64 // Illuminated fraction [0,1]: 0=new, 1=full
	AgeDays      float64 // Days since new moon [0,SynodicMonth)
	IsWaxing     bool    // True when moon is waxing (getting fuller)
	PhaseName    string  // Human-readable phase name
}

// Calculate computes the moon phase for a given UTC timestamp
func Calculate(t time.Time) MoonPhase {
	jd := jdFromTime(t)
	T := julianCenturies(jd)

	lambdaSun := sunEclipticLongitude(T)
	lambdaMoon := moonEclipticLongitude(T)

	elongation := normalizeAngle(lambdaMoon - lambdaSun)
	phase := elongation / 360.0
	illumination := (1 - math.Cos(degToRad(elongation))) / 2
	ageDays := phase * SynodicMonth
	isWaxing := elongation < 180

	return MoonPhase{
		Phase:        phase,
		Elongation:   elongation,
		Illumination: illumination,
		AgeDays:      ageDays,
		IsWaxing:     isWaxing,
		PhaseName:    phaseName(illumination, isWaxing),
	}
}

// phaseName returns the 8-phase name based on illumination percentage and direction
func phaseName(illumination float64, isWaxing bool) string {
	switch {
	case illumination < 0.01:
		return "New Moon"
	case illumination > 0.99:
		return "Full Moon"
	case illumination >= 0.49 && illumination <= 0.51:
		if isWaxing {
			return "First Quarter"
		}
		return "Third Quarter"
	case illumination < 0.50:
		if isWaxing {
			return "Waxing Crescent"
		}
		return "Waning Crescent"
	default:
		if isWaxing {
			return "Waxing Gibbous"
		}
		return "Waning Gibbous"
	}
}

// jdFromTime converts a UTC time to Julian Day
func jdFromTime(t time.Time) float64 {
	return 2440587.5 + float64(t.Unix())/86400.0
}

// julianCenturies returns Julian centuries since J2000.0
func julianCenturies(jd float64) float64 {
	return (jd - 2451545.0) / 36525.0
}

// normalizeAngle wraps an angle to the range [0, 360)
func normalizeAngle(angle float64) float64 {
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}
	return angle
}

// degToRad converts degrees to radians
func degToRad(deg float64) float64 {
	return deg * math.Pi / 180.0
}

// sunEclipticLongitude computes the Sun's ecliptic longitude in degrees
func sunEclipticLongitude(T float64) float64 {
	// Mean longitude
	L0 := 280.46646 + 36000.76983*T + 0.0003032*T*T

	// Mean anomaly
	M := 357.52911 + 35999.05029*T - 0.0001537*T*T
	Mrad := degToRad(normalizeAngle(M))

	// Equation of center
	C := (1.914602-0.004817*T-0.000014*T*T)*math.Sin(Mrad) +
		(0.019993-0.000101*T)*math.Sin(2*Mrad) +
		0.000289*math.Sin(3*Mrad)

	return normalizeAngle(L0 + C)
}

// moonEclipticLongitude computes the Moon's ecliptic longitude in degrees
func moonEclipticLongitude(T float64) float64 {
	// Mean longitude
	L := 218.3164477 +
		481267.88123421*T -
		0.0015786*T*T +
		T*T*T/538841 -
		T*T*T*T/65194000

	// Moon mean elongation
	D := 297.8501921 +
		445267.1114034*T -
		0.0018819*T*T +
		T*T*T/545868 -
		T*T*T*T/113065000

	// Moon mean anomaly
	Mp := 134.9633964 +
		477198.8675055*T +
		0.0087414*T*T +
		T*T*T/69699 -
		T*T*T*T/14712000

	// Normalize before using in trig functions
	Drad := degToRad(normalizeAngle(D))
	Mprad := degToRad(normalizeAngle(Mp))

	// Longitude correction (dominant terms)
	lambdaMoon := L +
		6.289*math.Sin(Mprad) +
		1.274*math.Sin(2*Drad-Mprad) +
		0.658*math.Sin(2*Drad) +
		0.214*math.Sin(2*Mprad) +
		0.110*math.Sin(Drad)

	return normalizeAngle(lambdaMoon)
}
