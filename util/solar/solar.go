package solar

import (
	"math"
	"time"

	"github.com/soniakeys/meeus/v3/julian"
)

// SolarResult stores calculated solar parameters
type SolarResult struct {
	Irradiance     float64 // W/m², clear-sky solar radiation
	EqOfTimeMin    float64 // minutes, equation of time
	DeclinationDeg float64 // degrees, solar declination
	AzimuthDeg     float64 // degrees, solar azimuth (0° north, clockwise)
	ElevationDeg   float64 // degrees, solar elevation
	CosZenith      float64 // cosine of zenith angle
	SunEarthDistKm float64 // km, Sun-Earth distance
}

// degToRad converts degrees to radians
func degToRad(deg float64) float64 { return deg * math.Pi / 180.0 }

// radToDeg converts radians to degrees
func radToDeg(rad float64) float64 { return rad * 180.0 / math.Pi }

// fixAngle normalizes angle to [0, 360)
func fixAngle(a float64) float64 { return a - 360.0*math.Floor(a/360.0) }

// CalculateSolarRadiationBras computes solar parameters using Bras method
// Inputs: lat (degrees N), lon (degrees W), altM (meters), ts (Unix UTC), nfac (turbidity)
func CalculateSolarRadiationBras(lat, lon, altM float64, ts int64, nfac float64) SolarResult {
	const solarConstant = 1367.0 // W/m²
	const auToKm = 149597870.7   // km/AU

	// Time setup
	t := time.Unix(ts, 0).UTC()
	jd := julian.TimeToJD(t)
	T := (jd - 2451545.0) / 36525.0 // centuries since J2000

	// Solar coordinates
	L0 := fixAngle(280.46646 + T*(36000.76983+T*0.0003032)) // mean longitude
	M := fixAngle(357.52911 + T*(35999.05029-T*0.0001537))  // mean anomaly
	e := 0.016708634 - T*(0.000042037+T*0.0000001267)       // eccentricity
	C := math.Sin(degToRad(M))*(1.914602-T*(0.004817+T*0.000014)) +
		math.Sin(degToRad(2*M))*(0.019993-T*0.000101) +
		math.Sin(degToRad(3*M))*0.000289 // center equation
	sunLong := L0 + C                                                   // true longitude
	Ω := 125.04 - 1934.136*T                                            // node longitude
	λ := sunLong - 0.00569 - 0.00478*math.Sin(degToRad(Ω))              // corrected longitude
	eps0 := 23 + (26+(21.448-T*(46.815+T*(0.00059-T*0.001813)))/60)/60  // obliquity
	δRad := math.Asin(math.Sin(degToRad(eps0)) * math.Sin(degToRad(λ))) // declination

	// Equation of time
	y := math.Tan(degToRad(eps0)/2) * math.Tan(degToRad(eps0)/2)
	eqTimeMin := radToDeg(y*math.Sin(degToRad(2*L0))-
		2*e*math.Sin(degToRad(M))+
		4*e*y*math.Sin(degToRad(M))*math.Cos(degToRad(2*L0))-
		0.5*y*y*math.Sin(degToRad(4*L0))-
		1.25*e*e*math.Sin(degToRad(2*M))) * 4

	// Hour angle
	utcMin := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60.0
	timeOffset := 4*lon + eqTimeMin
	tst := utcMin + timeOffset
	ha := tst/4 - 180
	haRad := degToRad(ha)

	// Zenith and elevation
	latRad := degToRad(lat)
	cosZen := math.Sin(latRad)*math.Sin(δRad) + math.Cos(latRad)*math.Cos(δRad)*math.Cos(haRad)
	zenRad := math.Acos(cosZen)
	zenDeg := radToDeg(zenRad)
	elDeg := 90 - zenDeg + 0.5667 // refraction correction

	// Return zero values (except EqT and Dec) if Sun is at/below horizon
	if elDeg <= 0 {
		return SolarResult{0.0, eqTimeMin, radToDeg(δRad), 0.0, elDeg, cosZen, 0.0}
	}

	// Azimuth
	azNum := math.Sin(δRad) - math.Sin(latRad)*cosZen
	azDen := math.Cos(latRad) * math.Sin(zenRad)
	azRad := math.Acos(azNum / azDen)
	azDeg := radToDeg(azRad)
	// Adjust azimuth for post-noon times (ha > 0)
	if ha > 0 {
		azDeg = 360 - azDeg
	}

	// Sun-Earth distance (libastro s_edist)
	M_rad := degToRad(M)
	e = 0.016708617 - T*(0.000042037+T*0.0000001236)
	E := M_rad + e*math.Sin(M_rad)*(1+e*math.Cos(M_rad))
	v := 2 * math.Atan(math.Sqrt((1+e)/(1-e))*math.Tan(E/2))
	r := (1 - e*e) / (1 + e*math.Cos(v))
	sunEarthDistKm := r * auToKm

	// Irradiance (Bras)
	io := cosZen * solarConstant / (r * r)
	m := 1.0 / (cosZen + 0.15*math.Pow(elDeg+3.885, -1.253))
	a1 := 0.128 - 0.054*math.Log(m)/math.Ln10
	sr := io * math.Exp(-nfac*a1*m)
	// Clamp negative irradiance to zero
	if sr < 0 {
		sr = 0.0
	}

	return SolarResult{sr, eqTimeMin, radToDeg(δRad), azDeg, elDeg, cosZen, sunEarthDistKm}
}
