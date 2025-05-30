package solar

import (
	"math"
	"time"
)

// Constants
const (
	solarConstant = 1361.0 // Solar constant in W/m², the average solar energy at the top of Earth's atmosphere
)

// degToRad converts an angle from degrees to radians for trigonometric calculations
func degToRad(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

// radToDeg converts an angle from radians to degrees for human-readable output
func radToDeg(rad float64) float64 {
	return rad * (180.0 / math.Pi)
}

// fixAngle normalizes an angle to the range [0, 360) degrees
func fixAngle(angle float64) float64 {
	return math.Mod(angle+360, 360)
}

// jdFromTime converts a UTC time to Julian Day, a continuous count of days since Jan 1, 4713 BCE
func jdFromTime(t time.Time) float64 {
	// Formula: JD = 2440587.5 (Unix epoch JD) + seconds since epoch / seconds per day
	return 2440587.5 + float64(t.Unix())/86400.0
}

// equationOfTime calculates the Equation of Time (EoT) in minutes, the difference between apparent and mean solar time
func equationOfTime(t time.Time) float64 {
	jd := jdFromTime(t)
	T := (jd - 2451545.0) / 36525.0 // Julian centuries since J2000.0 (Jan 1, 2000, 12:00 TT)

	// Solar coordinates for EoT calculation
	L0 := fixAngle(280.46646 + T*(36000.76983+T*0.0003032))            // Mean longitude of the Sun (degrees)
	M := fixAngle(357.52911 + T*(35999.05029-T*0.0001537))             // Mean anomaly of the Sun (degrees)
	e := 0.016708634 - T*(0.000042037+T*0.0000001267)                  // Eccentricity of Earth's orbit
	eps0 := 23 + (26+(21.448-T*(46.815+T*(0.00059-T*0.001813)))/60)/60 // Mean obliquity of the ecliptic (degrees)

	// Equation of Time: Combines obliquity and eccentricity effects
	// y approximates the effect of Earth's tilt; terms adjust for orbital variations
	y := math.Tan(degToRad(eps0)/2) * math.Tan(degToRad(eps0)/2)
	eqTimeMin := radToDeg(y*math.Sin(degToRad(2*L0))-
		2*e*math.Sin(degToRad(M))+
		4*e*y*math.Sin(degToRad(M))*math.Cos(degToRad(2*L0))-
		0.5*y*y*math.Sin(degToRad(4*L0))-
		1.25*e*e*math.Sin(degToRad(2*M))) * 4 // Convert to minutes (4 min/radian)

	return eqTimeMin
}

// calculateGHI computes Global Horizontal Irradiance (GHI) in W/m² using the Ineichen-Perez clear-sky model
func CalculateGHIIneichenPerez(t time.Time, latitude, longitude, altitude float64) float64 {
	// Day of the year (1-365 or 366) for seasonal solar position
	N := t.YearDay()

	// Solar declination (δ): Angle between Earth's equator and the Sun, in degrees
	// Approximated with a sinusoidal variation peaking at solstices
	delta := 23.45 * math.Sin(degToRad(360.0/365.0*float64(N-81)))

	// Hour angle (H): Angular distance of the Sun from local meridian, in degrees
	// Incorporates Equation of Time for true solar time
	utcMin := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60.0 // UTC time in minutes
	eqTimeMin := equationOfTime(t)                                       // EoT in minutes
	timeOffset := 4*longitude + eqTimeMin                                // Total offset: 4 min/deg longitude + EoT
	tst := utcMin + timeOffset                                           // True solar time in minutes
	H := (tst / 4) - 180                                                 // Hour angle: TST in degrees - 180° (noon = 0°)

	// Solar zenith angle (θ_z): Angle between vertical and Sun's position, in degrees
	latRad := degToRad(latitude)
	deltaRad := degToRad(delta)
	HRad := degToRad(H)
	cosThetaZ := math.Sin(latRad)*math.Sin(deltaRad) + math.Cos(latRad)*math.Cos(deltaRad)*math.Cos(HRad)
	thetaZ := radToDeg(math.Acos(cosThetaZ))

	// Extraterrestrial radiation (G0): Solar energy at the top of the atmosphere, in W/m²
	// Adjusted for Earth-Sun distance variation throughout the year
	G0 := solarConstant * (1 + 0.033*math.Cos(degToRad(360.0*(float64(N)-3)/365.0)))

	// Ineichen-Perez clear-sky model: Calculates GHI under clear-sky conditions
	if thetaZ < 90.0 { // Sun above horizon
		TL := 2.0 // Linke turbidity factor, typical for clear skies (range: 2-6)
		// Air mass (AM): Atmospheric path length, using Kasten-Young formula
		AM := 1.0 / (math.Cos(degToRad(thetaZ)) + 0.50572*math.Pow(96.07995-thetaZ, -1.6364))
		c := 0.7   // Normalization constant for DNI (tuned for accuracy)
		a := 0.027 // Atmospheric extinction coefficient
		// Direct Normal Irradiance (DNI): Direct beam radiation perpendicular to Sun
		DNI := G0 * c * math.Exp(-a*AM*TL*math.Exp(-altitude/8000.0))
		// Diffuse Horizontal Irradiance (DHI): Scattered radiation, seasonal adjustment
		fh := 0.1 + 0.05*math.Sin(math.Pi*float64(N-100)/365.0)
		DHI := fh * G0 * math.Sin(degToRad(thetaZ))
		// GHI: Total radiation on horizontal surface = direct + diffuse
		return DNI*math.Cos(degToRad(thetaZ)) + DHI
	}
	return 0.0 // Sun below horizon, no irradiance
}
