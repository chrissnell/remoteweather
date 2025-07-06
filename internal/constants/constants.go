// Package constants defines application-wide constants and version information.
package constants

import "runtime"

// Version holds the application version information
const Version = "3.0-" + runtime.GOOS + "/" + runtime.GOARCH
