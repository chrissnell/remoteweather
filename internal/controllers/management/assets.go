package management

import (
	"embed"
	"io/fs"
)

// Embed the management interface assets
//
//go:embed assets/**
var assetsFS embed.FS

// GetAssets returns the embedded assets filesystem
func GetAssets() fs.FS {
	// Return a sub-filesystem starting from the "assets" directory
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic("failed to create assets sub-filesystem: " + err.Error())
	}
	return assets
}
