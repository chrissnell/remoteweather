//go:build ignore

package main

// This file ensures all internal packages are included in the build
// when building the main remoteweather application.

import (
	_ "github.com/chrissnell/remoteweather/internal/controllers"
	_ "github.com/chrissnell/remoteweather/internal/storage"
	_ "github.com/chrissnell/remoteweather/internal/weatherstation" 
	_ "github.com/chrissnell/remoteweather/internal/core"
)
