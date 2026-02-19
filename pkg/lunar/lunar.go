// Package lunar provides moon phase calculations using ecliptic longitudes
// of the Sun and Moon. Accuracy is typically within ~0.5-1% illumination
// and ~1-2 hours of phase angle. Crescent angle calculations use full
// equatorial coordinates with parallactic angle correction for the observer.
package lunar

import (
	"math"
	"time"
)

// SynodicMonth is the average length of the lunar cycle in days
const SynodicMonth = 29.530588853

// CrescentAngle contains the full set of computed orientation values
type CrescentAngle struct {
	BrightLimbAngle float64 // χ: position angle of bright limb (degrees, from celestial N toward E)
	TerminatorAngle float64 // θ: terminator orientation in celestial coords (degrees)
	ParallacticAngle float64 // q: parallactic angle of the Moon (degrees)
	LocalTerminator float64 // θ_local: terminator angle relative to observer's local vertical (degrees)
	Rotation        float64 // CSS rotation to apply to the icon (degrees, clockwise positive)
	PhaseAngle      float64 // i: Sun-Moon-Earth angle (degrees)
	Illumination    float64 // k: illuminated fraction [0,1], computed from phase angle
}

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

// moonEclipticLatitude computes the Moon's ecliptic latitude in degrees.
// Uses the dominant terms from Meeus Ch. 47.
func moonEclipticLatitude(T float64) float64 {
	// Argument of latitude F
	F := 93.2720950 +
		483202.0175233*T -
		0.0036539*T*T -
		T*T*T/3526000 +
		T*T*T*T/863310000

	// Moon mean elongation (same as in moonEclipticLongitude)
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

	Frad := degToRad(normalizeAngle(F))
	Drad := degToRad(normalizeAngle(D))
	Mprad := degToRad(normalizeAngle(Mp))

	// Latitude (dominant terms from Meeus Table 47.B)
	beta := 5.128*math.Sin(Frad) +
		0.2806*math.Sin(Mprad+Frad) +
		0.2777*math.Sin(Mprad-Frad) +
		0.1732*math.Sin(2*Drad-Frad)

	return beta
}

// obliquity computes the mean obliquity of the ecliptic in degrees (IAU formula)
func obliquity(T float64) float64 {
	return 23.439291111 - 0.013004167*T - 0.00000164*T*T + 0.000000504*T*T*T
}

// eclipticToEquatorial converts ecliptic coordinates (lambda, beta in degrees)
// to equatorial coordinates (ra, dec in radians) given obliquity epsilon in degrees.
func eclipticToEquatorial(lambdaDeg, betaDeg, epsilonDeg float64) (ra, dec float64) {
	lam := degToRad(lambdaDeg)
	bet := degToRad(betaDeg)
	eps := degToRad(epsilonDeg)

	sinBet := math.Sin(bet)
	cosBet := math.Cos(bet)
	sinLam := math.Sin(lam)
	cosLam := math.Cos(lam)
	sinEps := math.Sin(eps)
	cosEps := math.Cos(eps)

	// Declination
	sinDec := sinBet*cosEps + cosBet*sinEps*sinLam
	dec = math.Asin(sinDec)

	// Right ascension
	y := sinLam*cosEps - math.Tan(bet)*sinEps
	x := cosLam
	ra = math.Atan2(y, x)
	if ra < 0 {
		ra += 2 * math.Pi
	}

	return ra, dec
}

// greenwichMeanSiderealTime computes GMST in degrees for a given Julian Day.
// Uses the IAU 1982 model (Meeus eq. 12.4).
func greenwichMeanSiderealTime(jd float64) float64 {
	// Julian day at preceding midnight
	jd0 := math.Floor(jd-0.5) + 0.5
	S := jd0 - 2451545.0
	T := S / 36525.0

	// GMST at midnight in hours
	gmst := 6.697374558 + 2400.0513369*T + 0.0000258622*T*T - 1.7222e-9*T*T*T

	// Hours elapsed since midnight UT
	ut := (jd - jd0) * 24.0
	gmst += 1.00273790935 * ut

	// Normalize to [0, 24) hours then convert to degrees
	gmst = math.Mod(gmst, 24)
	if gmst < 0 {
		gmst += 24
	}
	return gmst * 15.0 // hours to degrees
}

// localSiderealTime computes the local sidereal time in radians
func localSiderealTime(jd, lonDeg float64) float64 {
	gmstDeg := greenwichMeanSiderealTime(jd)
	lstDeg := normalizeAngle(gmstDeg + lonDeg)
	return degToRad(lstDeg)
}

func radToDeg(rad float64) float64 {
	return rad * 180.0 / math.Pi
}

// normalizeRadians wraps an angle in radians to the range [0, 2π)
func normalizeRadians(angle float64) float64 {
	twoPi := 2 * math.Pi
	angle = math.Mod(angle, twoPi)
	if angle < 0 {
		angle += twoPi
	}
	return angle
}

// CalculateCrescentAngle computes the full crescent orientation for an observer.
// Returns a CrescentAngle with the rotation angle to apply to a moon phase icon
// so that the terminator matches the real observed orientation in the sky.
//
// latDeg and lonDeg are the observer's geographic latitude and longitude in degrees
// (east positive). If both are zero, the parallactic angle correction is skipped
// and the geocentric terminator angle is returned.
func CalculateCrescentAngle(t time.Time, latDeg, lonDeg float64) CrescentAngle {
	jd := jdFromTime(t)
	T := julianCenturies(jd)

	// Sun ecliptic coordinates (β_sun ≈ 0)
	lambdaSun := sunEclipticLongitude(T)
	raSun, decSun := eclipticToEquatorial(lambdaSun, 0, obliquity(T))

	// Moon ecliptic coordinates (need latitude for accurate equatorial conversion)
	lambdaMoon := moonEclipticLongitude(T)
	betaMoon := moonEclipticLatitude(T)
	eps := obliquity(T)
	raMoon, decMoon := eclipticToEquatorial(lambdaMoon, betaMoon, eps)

	// Elongation E via spherical law of cosines
	cosE := math.Sin(decMoon)*math.Sin(decSun) +
		math.Cos(decMoon)*math.Cos(decSun)*math.Cos(raMoon-raSun)
	// Clamp to [-1, 1] for numerical safety
	if cosE > 1 {
		cosE = 1
	} else if cosE < -1 {
		cosE = -1
	}
	E := math.Acos(cosE)

	// Phase angle i ≈ π - E (distance-based formula more precise but
	// requires Sun-Moon distance which we don't compute)
	phaseAngle := math.Pi - E

	// Illuminated fraction k
	k := (1 + math.Cos(phaseAngle)) / 2

	// Position angle of the bright limb χ (Meeus eq. 48.5)
	// Direction from Moon center toward Sun, measured from celestial N toward E
	deltaRA := raSun - raMoon
	chiY := math.Cos(decSun) * math.Sin(deltaRA)
	chiX := math.Sin(decSun)*math.Cos(decMoon) - math.Cos(decSun)*math.Sin(decMoon)*math.Cos(deltaRA)
	chi := math.Atan2(chiY, chiX)
	chi = normalizeRadians(chi)

	// Terminator angle θ (perpendicular to bright limb direction)
	theta := normalizeRadians(chi + math.Pi/2)

	result := CrescentAngle{
		BrightLimbAngle: radToDeg(chi),
		TerminatorAngle: radToDeg(theta),
		PhaseAngle:      radToDeg(phaseAngle),
		Illumination:    k,
	}

	// Parallactic angle q: rotation between celestial N and local zenith
	// as seen at the Moon's position. Skip if no location configured.
	if latDeg == 0 && lonDeg == 0 {
		result.Rotation = radToDeg(-theta)
		result.LocalTerminator = radToDeg(theta)
		return result
	}

	phi := degToRad(latDeg)
	lst := localSiderealTime(jd, lonDeg)
	H := lst - raMoon // hour angle

	qY := math.Sin(H)
	qX := math.Tan(phi)*math.Cos(decMoon) - math.Sin(decMoon)*math.Cos(H)
	q := math.Atan2(qY, qX)

	thetaLocal := normalizeRadians(theta - q)

	result.ParallacticAngle = radToDeg(q)
	result.LocalTerminator = radToDeg(thetaLocal)
	// CSS clockwise rotation with 0 pointing up: negate θ_local
	result.Rotation = radToDeg(-thetaLocal)

	return result
}
