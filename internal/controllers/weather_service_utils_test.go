package controllers

import (
	"testing"

	"github.com/chrissnell/remoteweather/internal/log"
)

func TestMain(m *testing.M) {
	// ResolveUploadInterval logs via the package-level logger when clamping, so
	// the global logger must be initialized before tests run.
	_ = log.Init(false)
	m.Run()
}

func TestResolveUploadInterval(t *testing.T) {
	cases := []struct {
		name                    string
		configured, def, minVal int
		want                    int
	}{
		{"unset uses default", 0, 60, MinPWSUploadInterval, 60},
		{"configured above minimum", 120, 60, MinPWSUploadInterval, 120},
		{"configured below minimum clamps up", 10, 60, MinPWSUploadInterval, MinPWSUploadInterval},
		{"wu unset uses 300 default", 0, 300, MinWUUploadInterval, 300},
		{"negative treated as unset", -5, 300, MinWUUploadInterval, 300},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveUploadInterval(c.configured, c.def, c.minVal, "Test Service")
			if got != c.want {
				t.Errorf("ResolveUploadInterval(%d,%d,%d) = %d, want %d",
					c.configured, c.def, c.minVal, got, c.want)
			}
		})
	}
}
