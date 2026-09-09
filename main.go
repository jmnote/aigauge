package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"

	usageapp "github.com/jmnote/aigauge/internal/app"
	"github.com/jmnote/aigauge/internal/ui"
)

//go:embed frontend
var embeddedFrontend embed.FS

//go:embed frontend/logo.png
var appIcon []byte

func main() {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--theme=") {
			theme := strings.TrimPrefix(arg, "--theme=")
			if theme == "light" || theme == "dark" || theme == "system" {
				usageapp.ThemeOverride = theme
			}
		}
	}
	frontendAssets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal("failed to initialize embedded frontend assets: ", err)
	}
	if err := ui.Run(frontendAssets, appIcon); err != nil {
		log.Fatal(err)
	}
}
