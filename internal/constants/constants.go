package constants

import "runtime"

// Version holds the application version information
const Version = "3.0-" + runtime.GOOS + "/" + runtime.GOARCH
