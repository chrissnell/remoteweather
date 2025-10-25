// Package weatherstations provides capability definitions for weather station types.
package weatherstations

import "strings"

// Capability represents a specific measurement capability of a weather station.
// Capabilities use a bitmask to allow stations to have multiple capabilities.
type Capability uint8

const (
	// Weather represents standard meteorological measurements:
	// temperature, humidity, pressure, wind speed/direction, rainfall
	Weather Capability = 1 << 0 // 0x01

	// Snow represents snow depth and accumulation measurements
	Snow Capability = 1 << 1 // 0x02

	// AirQuality represents air quality measurements:
	// PM2.5, PM10, CO2, VOC, NOx
	AirQuality Capability = 1 << 2 // 0x04
)

// String returns the human-readable name of a capability.
func (c Capability) String() string {
	switch c {
	case Weather:
		return "Weather"
	case Snow:
		return "Snow"
	case AirQuality:
		return "AirQuality"
	default:
		return "Unknown"
	}
}

// Capabilities represents a set of capabilities using a bitmask.
// This allows efficient storage and checking of multiple capabilities.
type Capabilities uint8

// Has checks if a specific capability is present in the set.
func (c Capabilities) Has(cap Capability) bool {
	return (uint8(c) & uint8(cap)) != 0
}

// Add adds a capability to the set.
func (c *Capabilities) Add(cap Capability) {
	*c = Capabilities(uint8(*c) | uint8(cap))
}

// Remove removes a capability from the set.
func (c *Capabilities) Remove(cap Capability) {
	*c = Capabilities(uint8(*c) &^ uint8(cap))
}

// List returns all capabilities in the set as a slice.
func (c Capabilities) List() []Capability {
	var caps []Capability
	if c.Has(Weather) {
		caps = append(caps, Weather)
	}
	if c.Has(Snow) {
		caps = append(caps, Snow)
	}
	if c.Has(AirQuality) {
		caps = append(caps, AirQuality)
	}
	return caps
}

// String returns a comma-separated string of all capabilities in the set.
func (c Capabilities) String() string {
	caps := c.List()
	if len(caps) == 0 {
		return "None"
	}

	strs := make([]string, len(caps))
	for i, cap := range caps {
		strs[i] = cap.String()
	}
	return strings.Join(strs, ", ")
}

// IsEmpty returns true if no capabilities are set.
func (c Capabilities) IsEmpty() bool {
	return uint8(c) == 0
}

// Count returns the number of capabilities in the set.
func (c Capabilities) Count() int {
	return len(c.List())
}
