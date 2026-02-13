package solar

import (
	"math"
	"time"
)

// CalculateSunriseSunset returns sunrise and sunset as minutes from midnight UTC
// for the given day-of-year at the specified latitude and longitude.
// Returns (-1, -1, nil) for polar day (sun never sets) or polar night (sun never rises).
func CalculateSunriseSunset(dayOfYear int, latitude, longitude float64) (sunriseMinutes, sunsetMinutes int, err error) {
	// Solar declination using the formula from asce.go
	// This gives the angle between the Sun and the celestial equator
	doy := float64(dayOfYear)
	innerAngle := (356.6 + 0.9856*doy) * (math.Pi / 180.0)
	outerAngle := (278.97 + 0.9856*doy + 1.9165*math.Sin(innerAngle)) * (math.Pi / 180.0)
	declinationRad := math.Asin(0.39785 * math.Sin(outerAngle))

	// Convert latitude to radians
	latRad := latitude * (math.Pi / 180.0)

	// Calculate the hour angle at sunrise/sunset
	// At sunrise/sunset, the sun is at the horizon (zenith angle = 90°)
	// cos(H) = -tan(lat) * tan(declination)
	cosH := -math.Tan(latRad) * math.Tan(declinationRad)

	// Check for polar day/night conditions
	if cosH < -1.0 {
		// Sun never sets (midnight sun / polar day)
		return -1, -1, nil
	}
	if cosH > 1.0 {
		// Sun never rises (polar night)
		return -1, -1, nil
	}

	// Hour angle in radians, then convert to hours
	hourAngleRad := math.Acos(cosH)
	hourAngleHours := hourAngleRad * (180.0 / math.Pi) / 15.0 // 15 degrees per hour

	// Solar noon in UTC is affected by longitude
	// Each degree of longitude = 4 minutes of time
	// Positive longitude (east) means earlier UTC time
	longitudeMinutes := longitude * 4.0

	// Calculate equation of time for this day
	// Use a reference time at noon UTC for the given day of year
	refTime := time.Date(time.Now().Year(), 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, dayOfYear-1)
	eotMinutes := equationOfTime(refTime)

	// Solar noon in UTC minutes from midnight
	// 720 = 12:00 UTC, adjusted for longitude and equation of time
	solarNoonUTC := 720.0 - longitudeMinutes - eotMinutes

	// Convert hour angle to minutes
	hourAngleMinutes := hourAngleHours * 60.0

	// Sunrise and sunset times in UTC minutes from midnight
	sunriseUTC := solarNoonUTC - hourAngleMinutes
	sunsetUTC := solarNoonUTC + hourAngleMinutes

	// Normalize to 0-1440 range (minutes in a day)
	sunriseUTC = math.Mod(sunriseUTC+1440, 1440)
	sunsetUTC = math.Mod(sunsetUTC+1440, 1440)

	return int(math.Round(sunriseUTC)), int(math.Round(sunsetUTC)), nil
}

// FormatSunTime converts UTC minutes from midnight to a formatted time string
// in the given timezone location.
func FormatSunTime(utcMinutes int, loc *time.Location) string {
	if utcMinutes < 0 {
		return ""
	}

	hours := utcMinutes / 60
	minutes := utcMinutes % 60

	// Create a time in UTC, then convert to local
	t := time.Date(2000, 1, 1, hours, minutes, 0, 0, time.UTC)
	local := t.In(loc)

	return local.Format("3:04 PM")
}
