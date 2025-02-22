// License: MIT

// Unit defines some types used in astronomy, and heavily used by the packages
// in github.com/soniakeys/meeus.  Four types are currently defined, Angle,
// HourAngle, RA, and Time.  All are commonly formatted in sexagesimal notation
// and the external package github.com/soniakeys/sexagesimal has formatting
// routines.  Routines here are methods on the four types and a few other
// related functions.
//
// The underlying type for these four types is simply float64.  For Angle,
// HourAngle, and RA, the value is always in radians.  For Time, it is seconds.
//
// The choice of methods defined is somewhat arbitrary.  Methods were defined
// as they were found convenient for the Meeus library, and then filled out
// somewhat for consistency.  The convenience is often syntactic; as the
// underlying type is float64 conversions to and from float64 are often free
// and otherwise typically only incur a multiplication.
package unit

import "math"

// FromSexa converts from parsed sexagesimal angle components to a single
// float64 value.
//
// The result is in the units of d, the first or "largest" sexagesimal
// component.
//
// Typically you pass non-negative values for d, m, and s, and to indicate
// a negative value, pass '-' for neg.  Any other value, such as ' ', '+',
// or simply 0, leaves the result non-negative.
//
// There are no limits on d, m, or s however.  Negative values or values
// > 60 for m and s are allowed for example.  The segment values are
// combined and then if neg is '-' that sum is negated.
//
// This function would commonly be called something like DMSToDegrees, but
// the interpretation of d as degrees is arbitrary.  The function works
// as well on hours minutes and seconds.  Regardless of the units of d,
// m is a sexagesimal part of d and s is a sexagesimal part of m.
func FromSexa(neg byte, d, m int, s float64) float64 {
	return FromSexaSec(neg, d, m, s) / 3600
}

// FromSexaSec converts from parsed sexagesimal angle components to a single
// float64 value.
//
// The result is in the units of s, the last or "smallest" sexagesimal
// component.
//
// Otherwise FromSexaSec works as FromSexa.  See FromSexa.
func FromSexaSec(neg byte, d, m int, s float64) float64 {
	s = (float64((d*60+m)*60) + s)
	if neg == '-' {
		return -s
	}
	return s
}

// PMod returns a positive floating-point x mod y for a positive y.
//
// Argument x can be positive or negative, but y should be positive.
// With this restriction on y, PMod returns a value in the range [0,y).
//
// The method is intended only for positive y.  If y is negative, the result
// is not particularly useful.
func PMod(x, y float64) float64 {
	r := math.Mod(x, y)
	if r < 0 {
		r += y
	}
	return r
}

// Angle represents a general purpose angle.
//
// The value is stored as radians.
//
// There is no "AngleFromRad" constructor.  If you have a value `rad` in
// radians, construct a corresponding Angle simply with the Go type conversion
// `unit.Angle(rad)`.
type Angle float64

// ---------- Angle constructors: ----------

// AngleFromDeg constructs an Angle value from a value representing angular
// degrees where there are 360 degrees to a circle or revolution.
//
// This provides "Deg2Rad" functionality but hopefully in a more clear way.
func AngleFromDeg(d float64) Angle {
	// 180 deg or pi radians in a half-circle.
	return Angle(d / 180 * math.Pi)
}

// AngleFromMin constructs an Angle value from a value representing angular
// minutes where there are 60 minutes to a degree, and 360 degrees to a circle.
func AngleFromMin(m float64) Angle {
	// 60 min in a degree, 180 deg or pi radians in a half-circle.
	return Angle(m / 60 / 180 * math.Pi)
}

// AngleFromSec constructs an Angle value from a value representing angular
// seconds where there are 60 seconds to a minute, 60 minutes to a degree,
// and 360 degrees to a circle.
func AngleFromSec(s float64) Angle {
	// 3600 sec in a degree, 180 deg or pi radians in a half-circle.
	return Angle(s / 3600 / 180 * math.Pi)
}

// NewAngle constructs a new Angle value from sign, degree, minute, and second
// components.
//
// For argument neg, pass '-' to negate the result.  Any other argument
// value, such as ' ', '+', or simply 0, leaves the result non-negated.
func NewAngle(neg byte, d, m int, s float64) Angle {
	return AngleFromSec(FromSexaSec(neg, d, m, s))
}

// ---------- Angle "getters" or conversions: ----------

// Rad returns the angle in radians.
//
// This is the underlying representation and involves no scaling.
func (a Angle) Rad() float64 { return float64(a) }

// Deg returns the angle in degrees.
//
// This provides a "Rad2Deg" functionality but hopefully in a more clear way.
func (a Angle) Deg() float64 { return float64(a) * 180 / math.Pi }

// Min returns the angle in minutes.
func (a Angle) Min() float64 { return float64(a) * 60 * 180 / math.Pi }

// Sec returns the angle in seconds.
func (a Angle) Sec() float64 { return float64(a) * 3600 * 180 / math.Pi }

// HourAngle constructs an HourAngle value corresponding to angle a.
//
// As both types represent angles in radians, this is a zero-cost conversion.
func (a Angle) HourAngle() HourAngle { return HourAngle(a) }

// RA constructs an RA value corresponding to angle a.
//
// As usual for right ascension, the value is wrapped to the range [0,24h).
func (a Angle) RA() RA { return RAFromRad(a.Rad()) }

// Time constructs a Time value where one circle of Angle corresponds to
// one day of Time.
func (a Angle) Time() Time { return TimeFromRad(a.Rad()) }

// ---------- Angle math: ----------

// Mul returns the scalar product a*f
func (a Angle) Mul(f float64) Angle { return a * Angle(f) }

// Div returns the scalar quotient a/d
func (a Angle) Div(d float64) Angle { return a / Angle(d) }

// Mod1 returns Angle a wrapped to 1 circle.
func (a Angle) Mod1() Angle { return Angle(PMod(a.Rad(), 2*math.Pi)) }

// Sin returns the trigonometric sine of a.
func (a Angle) Sin() float64 { return math.Sin(a.Rad()) }

// Cos returns the trigonometric cosine of a.
func (a Angle) Cos() float64 { return math.Cos(a.Rad()) }

// Tan returns the trigonometric tangent of a.
func (a Angle) Tan() float64 { return math.Tan(a.Rad()) }

// Sincos returns the trigonometric sine and cosine of a.
func (a Angle) Sincos() (float64, float64) { return math.Sincos(a.Rad()) }

// HourAngle represents an angle corresponding to angular rotation of
// the Earth.
//
// The value is stored as radians.
type HourAngle float64

// ---------- HourAngle constructors: ----------

// HourAngleFromHour constructs an HourAngle value from a value representing
// hours of rotation or revolution where there are 24 hours to a revolution.
func HourAngleFromHour(h float64) HourAngle {
	// 12 hours or pi radians in a half-revolution
	return HourAngle(h / 12 * math.Pi)
}

// HourAngleFromMin constructs an HourAngle value from a value representing
// minutes of revolution where there are 60 minutes to an hour and 24 hours
// to a revolution.
func HourAngleFromMin(m float64) HourAngle {
	// 60 sec in an hour, 12 hours or pi radians in a half-revolution
	return HourAngle(m / 60 / 12 * math.Pi)
}

// HourAngleFromSec constructs an HourAngle value from a value representing
// seconds of revolution where there are 60 seconds to a minute, 60 minutes
// to an hour, and 24 hours to a revolution.
func HourAngleFromSec(s float64) HourAngle {
	// 3600 sec in an hour, 12 hours or pi radians in a half-revolution
	return HourAngle(s / 3600 / 12 * math.Pi)
}

// NewHourAngle constructs a new HourAngle value from sign, hour, minute,
// and second components.
//
// For argument neg, pass '-' to indicate a negative hour angle.  Any other
// argument value, such as ' ', '+', or simply 0, leaves the result
// non-negative.
func NewHourAngle(neg byte, h, m int, s float64) HourAngle {
	return HourAngle(FromSexa(neg, h, m, s) / 12 * math.Pi)
}

// ---------- HourAngle "getters" or conversions: ----------

// Rad returns the hour angle as an angle in radians.
//
// This is the underlying representation and involves no scaling.
func (h HourAngle) Rad() float64 { return float64(h) }

// Hour returns the hour angle as hours of revolution.
func (h HourAngle) Hour() float64 { return float64(h) * 12 / math.Pi }

// Min returns the hour angle as minutes of revolution.
func (h HourAngle) Min() float64 { return float64(h) * 60 * 12 / math.Pi }

// Sec returns the hour angle as seconds of revolution.
func (h HourAngle) Sec() float64 { return float64(h) * 3600 * 12 / math.Pi }

// Angle returns an Angle value where one revolution or 24 hours of HourAngle
// corresponds to one circle of Angle.
func (h HourAngle) Angle() Angle { return Angle(h) }

// RA returns an RA value corresponding to h but wrapped to the range [0,24h).
func (h HourAngle) RA() RA { return RAFromRad(h.Rad()) }

// Time returns a Time value where one revolution or 24 hours or HourAngle
// corresponds to one day of Time.
func (h HourAngle) Time() Time { return Time(h.Sec()) }

// ---------- Hour angle math: ----------

// Mul returns the scalar product h*f
func (h HourAngle) Mul(f float64) HourAngle { return h * HourAngle(f) }

// Div returns the scalar quotient h/f
func (h HourAngle) Div(f float64) HourAngle { return h / HourAngle(f) }

// Sin returns the trigonometric sine of h.
func (h HourAngle) Sin() float64 { return math.Sin(h.Rad()) }

// Cos returns the trigonometric cosine of h.
func (h HourAngle) Cos() float64 { return math.Cos(h.Rad()) }

// Tan returns the trigonometric tangent of h.
func (h HourAngle) Tan() float64 { return math.Tan(h.Rad()) }

// Sincos returns the trigonometric sine and cosine of h.
func (h HourAngle) Sincos() (float64, float64) { return math.Sincos(h.Rad()) }

// RA represents a value of right ascension.
//
// The value is stored as radians.
type RA float64

// ---------- RA constructors: ----------

// NewRA constructs a new RA value from hour, minute, and second components.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func NewRA(h, m int, s float64) RA {
	return RAFromRad(FromSexa(0, h, m, s) / 12 * math.Pi)
}

// RAFromDeg constructs an RA value from a value representing degrees of right
// ascension where there are 360 degrees to a circle or revolution.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func RAFromDeg(d float64) RA { return RAFromRad(d / 180 * math.Pi) }

// RAFromHour constructs an RA value from a value representing
// hours of RA where there are 24 hours to a revolution.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func RAFromHour(h float64) RA { return RAFromRad(h / 12 * math.Pi) }

// RAFromMin constructs an RA value from a value representing minutes of RA
// where there are 60 minutes to an hour and 24 hours to a revolution.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func RAFromMin(m float64) RA { return RAFromRad(m / 60 / 12 * math.Pi) }

// RAFromRad constructs a new RA value from radians.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func RAFromRad(rad float64) RA { return RA(PMod(rad, 2*math.Pi)) }

// RAFromSec constructs an RA value from a value representing seconds of RA
// where there are 60 seconds to a minute, 60 minutes to an hour, and 24 hours
// to a revolution.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func RAFromSec(s float64) RA { return RAFromRad(s / 3600 / 12 * math.Pi) }

// ---------- RA "getters" or conversions: ----------

// Rad returns the RA as an angle in radians.
//
// This is the underlying representation and involves no scaling.
func (ra RA) Rad() float64 { return float64(ra) }

// Deg returns the RA as degrees of RA.
func (ra RA) Deg() float64 { return float64(ra) * 180 / math.Pi }

// Hour returns the RA as hours of RA.
func (ra RA) Hour() float64 { return float64(ra) * 12 / math.Pi }

// Min returns the RA as minutes of RA.
func (ra RA) Min() float64 { return float64(ra) * 60 * 12 / math.Pi }

// Sec returns the RA as seconds of RA.
func (ra RA) Sec() float64 { return float64(ra) * 3600 * 12 / math.Pi }

// Angle returns an Angle value where 24 hours or one revolution of RA
// corresponds to one circle of Angle.
//
// As both types represent angles in radians, this is a zero-cost conversion.
func (ra RA) Angle() Angle { return Angle(ra) }

// HourAngle constructs an HourAngle value corresponding to RA ra.
//
// As both types represent angles in radians, this is a zero-cost conversion.
func (ra RA) HourAngle() HourAngle { return HourAngle(ra) }

// Time constructs a Time value where 24 hours or one revolution of RA
// corresponds to one day of Time.
func (ra RA) Time() Time { return TimeFromRad(ra.Rad()) }

// ---------- RA math: ----------

// Add adds hour angle h to RA ra giving a new RA value.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func (ra RA) Add(h HourAngle) RA { return RAFromRad(ra.Rad() + h.Rad()) }

// Sin returns the trigonometric sine of ra.
func (ra RA) Sin() float64 { return math.Sin(ra.Rad()) }

// Cos returns the trigonometric cosine of ra.
func (ra RA) Cos() float64 { return math.Cos(ra.Rad()) }

// Tan returns the trigonometric tangent of ra.
func (ra RA) Tan() float64 { return math.Tan(ra.Rad()) }

// Sincos returns the trigonometric sine and cosineof ra.
func (ra RA) Sincos() (float64, float64) { return math.Sincos(ra.Rad()) }

// Time represents a duration or relative time.
//
// The value is stored as seconds.
//
// There is no "TimeFromSec" constructor. If you have a value `sec` in seconds,
// construct a corresponding Time value simply with the Go type conversion
// `unit.Time(sec)`.
type Time float64

// ---------- Time constructors: ----------

// NewTime constructs a new Time value from sign, hour, minute, and
// second components.
//
// For argument neg, pass '-' to indicate a negative time such as a negative
// delta.  Any other argument value, such as ' ', '+', or simply 0, leaves
// the result non-negative.
func NewTime(neg byte, h, m int, s float64) Time {
	s += float64((h*60 + m) * 60)
	if neg == '-' {
		return Time(-s)
	}
	return Time(s)
}

// TimeFromDay constructs a Time value from a value representing days.
func TimeFromDay(d float64) Time {
	// 3600 sec in an hour, 24 hours in a day
	return Time(d * 3600 * 24)
}

// TimeFromHour constructs a Time value from a value representing hours
// of time.
func TimeFromHour(h float64) Time {
	// 3600 sec in an hour
	return Time(h * 3600)
}

// TimeFromMin constructs a Time value from a value representing minutes
// of time.
func TimeFromMin(m float64) Time {
	// 60 sec in a min
	return Time(m * 60)
}

// TimeFromRad constructs a Time value from radians where 2 pi radians
// corresponds to one day.
func TimeFromRad(rad float64) Time {
	// 3600 sec in an hour, 12 hours or pi radians in a half-day
	return Time(rad * 3600 * 12 / math.Pi)
}

// ---------- Time "getters" or conversions: ----------

// Day returns time in days.
func (t Time) Day() float64 { return float64(t) / 3600 / 24 }

// Hour returns time in hours.
func (t Time) Hour() float64 { return float64(t) / 3600 }

// Min returns time in minutes.
func (t Time) Min() float64 { return float64(t) / 60 }

// Rad returns time in radians, where 1 day = 2 Pi radians of rotation.
func (t Time) Rad() float64 { return float64(t) / 3600 / 12 * math.Pi }

// Sec returns the time in seconds.
//
// This is the underlying representation and involves no scaling.
func (t Time) Sec() float64 { return float64(t) }

// Angle returns time t as an equivalent angle where 1 day = 2 Pi radians.
func (t Time) Angle() Angle { return Angle(t.Rad()) }

// HourAngle returns time t as an equivalent hour angle where
// 1 day = 2 Pi radians or 24 hours of HourAngle.
func (t Time) HourAngle() HourAngle { return HourAngle(t.Rad()) }

// RA returns time t as an equivalent RA where 1 day = 24 hours of RA.
//
// The result is wrapped to the range [0,2π), or [0,24) hours.
func (t Time) RA() RA { return RAFromRad(t.Rad()) }

// ---------- Time math: ----------

// Mul returns the scalar product t*f
func (t Time) Mul(f float64) Time { return Time(t.Sec() * f) }

// Div returns the scalar quotient t/d
func (t Time) Div(d float64) Time { return Time(t.Sec() / d) }

// Mod1 returns a new Time wrapped to one day, the range [0,86400) seconds.
func (t Time) Mod1() Time { return Time(PMod(float64(t), 3600*24)) }
