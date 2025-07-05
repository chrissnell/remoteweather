package restserver

import (
	"embed"
	"io/fs"
	"os"
)

// Embed the REST server assets
//
//go:embed all:assets
var assetsFS embed.FS

// GetAssets returns the assets filesystem, either from disk or embedded
func GetAssets() fs.FS {
	// If the REMOTEWEATHER_RESTSERVER_ASSETS_DIR environment variable is set and points to a
	// valid directory, serve assets directly from the file-system.  This is
	// useful during development because it removes the need to recompile the
	// binary every time a CSS/JS/HTML file is tweaked.
	if dir := os.Getenv("REMOTEWEATHER_RESTSERVER_ASSETS_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return os.DirFS(dir)
		}
	}

	// Return a sub-filesystem starting from the "assets" directory
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic("failed to create assets sub-filesystem: " + err.Error())
	}
	return assets
}
