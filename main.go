package main

import "log"

import (
	"embed"
	"io/fs"

	"github.com/jmnote/aigauge/internal/ui"
)

//go:embed frontend
var embeddedFrontend embed.FS

//go:embed frontend/icon.png
var appIcon []byte

func main() {
	frontendAssets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal("failed to initialize embedded frontend assets: ", err)
	}
	if err := ui.Run(frontendAssets, appIcon); err != nil {
		log.Fatal(err)
	}
}
