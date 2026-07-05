// Package kiss provides AX.25 UI frame construction and KISS framing for
// transmitting APRS packets to a TNC over serial or network connections.
package kiss

import (
	"fmt"
	"strconv"
	"strings"
)

// KISS protocol special bytes.
const (
	fend  = 0xC0 // frame delimiter
	fesc  = 0xDB // frame escape
	tfend = 0xDC // transposed frame end
	tfesc = 0xDD // transposed frame escape

	cmdData = 0x00 // KISS data frame on port 0

	controlUI = 0x03 // AX.25 unnumbered information frame
	pidNoL3   = 0xF0 // AX.25 protocol ID: no layer 3
)

// EncodeAX25UI builds an AX.25 UI (unnumbered information) frame carrying info,
// addressed from source to dest via the given digipeater path. Callsigns may
// include an SSID in the standard CALL-SSID form (e.g. "W1AW-13"); an SSID of 0
// may be written as "CALL" or "CALL-0".
func EncodeAX25UI(source, dest string, path []string, info []byte) ([]byte, error) {
	if source == "" {
		return nil, fmt.Errorf("kiss: source callsign is required")
	}
	if dest == "" {
		return nil, fmt.Errorf("kiss: destination callsign is required")
	}

	// Address order on the wire: destination, source, then digipeaters.
	addrs := make([]string, 0, 2+len(path))
	addrs = append(addrs, dest, source)
	for _, hop := range path {
		hop = strings.TrimSpace(hop)
		if hop != "" {
			addrs = append(addrs, hop)
		}
	}

	var frame []byte
	for i, a := range addrs {
		last := i == len(addrs)-1
		enc, err := encodeAddress(a, last)
		if err != nil {
			return nil, err
		}
		frame = append(frame, enc...)
	}

	frame = append(frame, controlUI, pidNoL3)
	frame = append(frame, info...)
	return frame, nil
}

// encodeAddress encodes a single AX.25 address field: 6 space-padded callsign
// bytes left-shifted one bit, followed by the SSID byte. When last is true the
// SSID byte's low bit (the HDLC address-extension bit) is set to mark the end of
// the address field list.
func encodeAddress(callsign string, last bool) ([]byte, error) {
	call, ssid, err := parseCallSSID(callsign)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 7)
	for i := 0; i < 6; i++ {
		c := byte(' ')
		if i < len(call) {
			c = call[i]
		}
		out[i] = c << 1
	}

	// SSID byte: bits 5-6 reserved (set to 1 per convention), bits 1-4 the SSID,
	// bit 0 the extension bit (1 on the final address field).
	ssidByte := byte(0x60) | (byte(ssid) << 1)
	if last {
		ssidByte |= 0x01
	}
	out[6] = ssidByte
	return out, nil
}

// parseCallSSID splits "CALL-SSID" into an uppercased callsign and numeric SSID.
func parseCallSSID(s string) (string, int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	call := s
	ssid := 0

	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		call = s[:idx]
		ssidStr := s[idx+1:]
		n, err := strconv.Atoi(ssidStr)
		if err != nil {
			return "", 0, fmt.Errorf("kiss: invalid SSID %q in %q", ssidStr, s)
		}
		ssid = n
	}

	if call == "" || len(call) > 6 {
		return "", 0, fmt.Errorf("kiss: invalid callsign %q (must be 1-6 characters)", call)
	}
	if ssid < 0 || ssid > 15 {
		return "", 0, fmt.Errorf("kiss: invalid SSID %d (must be 0-15)", ssid)
	}
	return call, ssid, nil
}

// EncodeFrame wraps a raw frame in a KISS data frame: FEND, command byte, the
// escaped payload, then FEND.
func EncodeFrame(frame []byte) []byte {
	out := make([]byte, 0, len(frame)+4)
	out = append(out, fend, cmdData)
	for _, b := range frame {
		switch b {
		case fend:
			out = append(out, fesc, tfend)
		case fesc:
			out = append(out, fesc, tfesc)
		default:
			out = append(out, b)
		}
	}
	out = append(out, fend)
	return out
}
