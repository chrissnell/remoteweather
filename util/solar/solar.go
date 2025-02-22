package solar

import (
	"math"
	"time"

	"github.com/soniakeys/meeus/v3/julian"
)

type SolarResult struct {
	Irradiance     float64
	EqOfTimeMin    float64
	DeclinationDeg float64
	AzimuthDeg     float64
	ElevationDeg   float64
	CosZenith      float64
	SunEarthDistKm float64
}

func degToRad(deg float64) float64 { return deg * math.Pi / 180.0 }
func radToDeg(rad float64) float64 { return rad * 180.0 / math.Pi }
func fixAngle(a float64) float64   { return a - 360.0*math.Floor(a/360.0) }

func CalculateSolarRadiationBras(lat, lon, altM float64, ts int64, nfac float64) SolarResult {
	const solarConstant = 1367.0
	const auToKm = 149597870.7

	t := time.Unix(ts, 0).UTC()
	jd := julian.TimeToJD(t)
	T := (jd - 2451545.0) / 36525.0

	L0 := fixAngle(280.46646 + T*(36000.76983+T*0.0003032))
	M := fixAngle(357.52911 + T*(35999.05029-T*0.0001537))
	e := 0.016708634 - T*(0.000042037+T*0.0000001267)
	C := math.Sin(degToRad(M))*(1.914602-T*(0.004817+T*0.000014)) +
		math.Sin(degToRad(2*M))*(0.019993-T*0.000101) +
		math.Sin(degToRad(3*M))*0.000289
	sunLong := L0 + C
	Ω := 125.04 - 1934.136*T
	λ := sunLong - 0.00569 - 0.00478*math.Sin(degToRad(Ω))
	eps0 := 23 + (26+(21.448-T*(46.815+T*(0.00059-T*0.001813)))/60)/60
	δRad := math.Asin(math.Sin(degToRad(eps0)) * math.Sin(degToRad(λ)))

	y := math.Tan(degToRad(eps0)/2) * math.Tan(degToRad(eps0)/2)
	eqTimeMin := radToDeg(y*math.Sin(degToRad(2*L0))-
		2*e*math.Sin(degToRad(M))+
		4*e*y*math.Sin(degToRad(M))*math.Cos(degToRad(2*L0))-
		0.5*y*y*math.Sin(degToRad(4*L0))-
		1.25*e*e*math.Sin(degToRad(2*M))) * 4

	utcMin := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60.0
	timeOffset := 4*lon + eqTimeMin
	tst := utcMin + timeOffset
	ha := tst/4 - 180
	haRad := degToRad(ha)

	latRad := degToRad(lat)
	cosZen := math.Sin(latRad)*math.Sin(δRad) + math.Cos(latRad)*math.Cos(δRad)*math.Cos(haRad)
	zenRad := math.Acos(cosZen)
	zenDeg := radToDeg(zenRad)
	elDeg := 90 - zenDeg + 0.5667

	if elDeg <= 0 {
		return SolarResult{
			Irradiance:     0.0,
			EqOfTimeMin:    eqTimeMin,
			DeclinationDeg: radToDeg(δRad),
			AzimuthDeg:     0.0,
			ElevationDeg:   elDeg,
			CosZenith:      cosZen,
			SunEarthDistKm: 0.0,
		}
	}

	azNum := math.Sin(δRad) - math.Sin(latRad)*cosZen
	azDen := math.Cos(latRad) * math.Sin(zenRad)
	azRad := math.Acos(azNum / azDen)
	azDeg := radToDeg(azRad)
	if ha > 0 {
		azDeg = 360 - azDeg
	}

	// Sun-Earth distance (libastro s_edist)
	M_rad := degToRad(M)
	e = 0.016708617 - T*(0.000042037+T*0.0000001236)
	E := M_rad + e*math.Sin(M_rad)*(1+e*math.Cos(M_rad))     // Declare E
	v := 2 * math.Atan(math.Sqrt((1+e)/(1-e))*math.Tan(E/2)) // Declare v
	r := (1 - e*e) / (1 + e*math.Cos(v))
	sunEarthDistKm := r * auToKm

	io := cosZen * solarConstant / (r * r)
	m := 1.0 / (cosZen + 0.15*math.Pow(elDeg+3.885, -1.253))
	a1 := 0.128 - 0.054*math.Log(m)/math.Ln10
	sr := io * math.Exp(-nfac*a1*m)
	if sr < 0 {
		sr = 0.0
	}

	return SolarResult{
		Irradiance:     sr,
		EqOfTimeMin:    eqTimeMin,
		DeclinationDeg: radToDeg(δRad),
		AzimuthDeg:     azDeg,
		ElevationDeg:   elDeg,
		CosZenith:      cosZen,
		SunEarthDistKm: sunEarthDistKm,
	}
}
