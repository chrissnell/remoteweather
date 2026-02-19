package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chrissnell/remoteweather/pkg/lunar"
)

func main() {
	var timeStr string
	flag.StringVar(&timeStr, "time", "", "UTC time to calculate phase for (RFC3339 format, e.g., 2024-01-15T12:00:00Z)")
	flag.Parse()

	var t time.Time
	if timeStr == "" {
		t = time.Now().UTC()
	} else {
		var err error
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing time: %v\n", err)
			os.Exit(1)
		}
	}

	phase := lunar.Calculate(t)

	fmt.Printf("Moon Phase for %s\n", t.Format(time.RFC3339))
	fmt.Printf("  Phase:        %.1f%% (%.4f)\n", phase.Phase*100, phase.Phase)
	fmt.Printf("  Phase Name:   %s\n", phase.PhaseName)
	fmt.Printf("  Illumination: %.1f%%\n", phase.Illumination*100)
	fmt.Printf("  Age:          %.1f days\n", phase.AgeDays)
	fmt.Printf("  Elongation:   %.1f°\n", phase.Elongation)
	if phase.IsWaxing {
		fmt.Printf("  Direction:    Waxing\n")
	} else {
		fmt.Printf("  Direction:    Waning\n")
	}
}
