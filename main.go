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

//go:embed frontend/images/logo.png
var appIcon []byte

func main() {
	startHidden := false
	explicitHidden := false
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--theme=") {
			theme := strings.TrimPrefix(arg, "--theme=")
			if theme == "light" || theme == "dark" || theme == "system" {
				usageapp.ThemeOverride = theme
			}
		}
		if arg == "--hidden" || arg == "--tray" || arg == "--minimized" {
			startHidden = true
			explicitHidden = true
		}
	}
	if !explicitHidden {
		var err error
		startHidden, err = usageapp.StartHiddenOnLaunch()
		if err != nil {
			log.Printf("failed to read startup activation: %v", err)
		}
	}

	frontendAssets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal("failed to initialize embedded frontend assets: ", err)
	}
	if err := ui.Run(frontendAssets, appIcon, startHidden); err != nil {
		log.Fatal(err)
	}
}
