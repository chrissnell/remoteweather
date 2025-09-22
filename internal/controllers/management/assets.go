package management

import (
	"embed"
	"io/fs"
	"os"
)

// Embed the management interface assets
//
//go:embed assets/**
var assetsFS embed.FS

// GetAssets returns the embedded assets filesystem
func GetAssets() (fs.FS, error) {
	// If the REMOTEWEATHER_MANAGEMENT_ASSETS_DIR environment variable is set and points to a
	// valid directory, serve assets directly from the file-system.  This is
	// useful during development because it removes the need to recompile the
	// binary every time a CSS/JS/HTML file is tweaked.
	if dir := os.Getenv("REMOTEWEATHER_MANAGEMENT_ASSETS_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return os.DirFS(dir), nil
		}
	}

	// Return a sub-filesystem starting from the "assets" directory
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, err
	}
	return assets, nil
}
