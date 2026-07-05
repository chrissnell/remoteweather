package kiss

import (
	"bytes"
	"testing"
)

func TestEncodeAX25UI(t *testing.T) {
	frame, err := EncodeAX25UI("W1AW-9", "APRS", []string{"WIDE1-1"}, []byte("!hello"))
	if err != nil {
		t.Fatalf("EncodeAX25UI: %v", err)
	}

	want := []byte{
		// dest "APRS" (SSID 0, not last)
		0x82, 0xA0, 0xA4, 0xA6, 0x40, 0x40, 0x60,
		// source "W1AW-9" (SSID 9, not last)
		0xAE, 0x62, 0x82, 0xAE, 0x40, 0x40, 0x72,
		// digi "WIDE1-1" (SSID 1, last)
		0xAE, 0x92, 0x88, 0x8A, 0x62, 0x40, 0x63,
		// control, PID
		0x03, 0xF0,
		// info
		'!', 'h', 'e', 'l', 'l', 'o',
	}

	if !bytes.Equal(frame, want) {
		t.Fatalf("frame mismatch:\n got %X\nwant %X", frame, want)
	}
}

func TestEncodeAX25UINoPath(t *testing.T) {
	frame, err := EncodeAX25UI("W1AW", "APRS", nil, []byte("x"))
	if err != nil {
		t.Fatalf("EncodeAX25UI: %v", err)
	}
	// Source is the last address, so its extension bit must be set.
	ssidByte := frame[13]
	if ssidByte&0x01 != 0x01 {
		t.Fatalf("expected extension bit set on last (source) address, got %02X", ssidByte)
	}
}

func TestEncodeAX25UIErrors(t *testing.T) {
	cases := []struct {
		name, source, dest string
	}{
		{"empty source", "", "APRS"},
		{"empty dest", "W1AW", ""},
		{"callsign too long", "TOOLONG", "APRS"},
		{"bad ssid", "W1AW-99", "APRS"},
		{"nonnumeric ssid", "W1AW-X", "APRS"},
	}
	for _, c := range cases {
		if _, err := EncodeAX25UI(c.source, c.dest, nil, []byte("x")); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestEncodeFrameEscaping(t *testing.T) {
	// Payload containing both FEND and FESC to exercise escaping.
	in := []byte{0x01, fend, 0x02, fesc, 0x03}
	got := EncodeFrame(in)
	want := []byte{
		fend, cmdData,
		0x01,
		fesc, tfend, // escaped FEND
		0x02,
		fesc, tfesc, // escaped FESC
		0x03,
		fend,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("frame mismatch:\n got %X\nwant %X", got, want)
	}
}

func TestEncodeFrameNoEscaping(t *testing.T) {
	got := EncodeFrame([]byte{0x41, 0x42})
	want := []byte{fend, cmdData, 0x41, 0x42, fend}
	if !bytes.Equal(got, want) {
		t.Fatalf("frame mismatch:\n got %X\nwant %X", got, want)
	}
}
