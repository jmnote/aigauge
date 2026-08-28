package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/jmnote/aigauge/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend
var embeddedFrontend embed.FS

//go:embed frontend/icon.png
var appIcon []byte

var frontendAssets fs.FS

var singleInstanceKey = [32]byte{
	0x61, 0x69, 0x67, 0x61, 0x75, 0x67, 0x65, 0x2d,
	0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2d,
	0x6b, 0x65, 0x79, 0x2d, 0x76, 0x31, 0x2d, 0x30,
}

func init() {
	var err error
	frontendAssets, err = fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		panic("failed to initialize embedded frontend assets: " + err.Error())
	}
}

func main() {
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name: "AI Gauge",
		Icon: appIcon,
		Services: []application.Service{
			application.NewService(&app.App{}),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.aigauge.app",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window != nil {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AI Gauge",
		Width:     300,
		Height:    500,
		Frameless: true,
		Windows: application.WindowsWindow{
			NonClientRegionSupport: true,
		},
	})
	showWindow := func() {
		window.Restore()
		window.Show()
		window.Focus()
	}
	placeTopRight := func() {
		screen := app.Screen.GetPrimary()
		if screen == nil {
			return
		}
		width, _ := window.Size()
		window.SetPosition(screen.WorkArea.X+screen.WorkArea.Width-width, screen.WorkArea.Y)
	}
	initialPlacement := true
	window.OnWindowEvent(events.Windows.WindowShow, func(_ *application.WindowEvent) {
		if initialPlacement {
			placeTopRight()
			initialPlacement = false
		}
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		if initialPlacement {
			placeTopRight()
			initialPlacement = false
		}
	})
	clampWindow := func() {
		screen, err := window.GetScreen()
		if err != nil || screen == nil {
			return
		}
		x, y := window.Position()
		width, height := window.Size()
		work := screen.WorkArea
		newX, newY := x, y
		if width >= work.Width {
			newX = work.X
		} else if x < work.X {
			newX = work.X
		} else if x+width > work.X+work.Width {
			newX = work.X + work.Width - width
		}
		if height >= work.Height {
			newY = work.Y
		} else if y < work.Y {
			newY = work.Y
		} else if y+height > work.Y+work.Height {
			newY = work.Y + work.Height - height
		}
		if newX == x && newY == y {
			return
		}
		window.SetPosition(newX, newY)
	}
	window.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
		clampWindow()
	})

	tray := app.SystemTray.New()
	tray.SetLabel("AI Gauge")
	tray.SetIcon(appIcon)
	tray.OnClick(func() {
		showWindow()
	})

	menu := app.NewMenu()
	menu.Add("Show").OnClick(func(_ *application.Context) {
		showWindow()
	})
	menu.Add("Exit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
