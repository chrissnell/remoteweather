package aprs

import (
	"strings"
)

// CalculatePasscode calculates the APRS-IS passcode for a given callsign
// using the pairwise XOR algorithm
func CalculatePasscode(callsign string) int {
	// Prepare the callsign: use only base callsign (no SSID) and convert to uppercase
	base := strings.ToUpper(strings.Split(callsign, "-")[0])

	// Initialize with 0x73E2
	code := 0x73E2

	// Process characters in pairs
	for i := 0; i < len(base); i += 2 {
		c1 := base[i]
		var c2 byte = 0
		if i+1 < len(base) {
			c2 = base[i+1]
		}
		code ^= int(c1) << 8
		code ^= int(c2)
	}

	return code & 0x7FFF
}

// CalculatePasscodeDebug shows the step-by-step calculation for debugging
func CalculatePasscodeDebug(callsign string) int {
	// Prepare the callsign: use only base callsign (no SSID) and convert to uppercase
	base := strings.ToUpper(strings.Split(callsign, "-")[0])

	println("Calculating passcode for:", base)

	// Initialize with 0x73E2
	code := 0x73E2
	println("Initial code: 0x73E2 =", code)

	// Process characters in pairs
	for i := 0; i < len(base); i += 2 {
		c1 := base[i]
		var c2 byte = 0
		if i+1 < len(base) {
			c2 = base[i+1]
		}

		println("Pair", (i/2)+1, ":", string(c1)+string(c2))
		println("  c1:", string(c1), "=", int(c1))
		println("  c2:", string(c2), "=", int(c2))

		oldCode := code
		code ^= int(c1) << 8
		println("  code ^= (c1 << 8) =", oldCode, "^", (int(c1) << 8), "=", code)

		oldCode = code
		code ^= int(c2)
		println("  code ^= c2 =", oldCode, "^", int(c2), "=", code)
		println()
	}

	result := code & 0x7FFF
	println("Final code:", code)
	println("Masked result (code & 0x7FFF):", result)
	return result
}
