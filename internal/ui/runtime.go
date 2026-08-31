package ui

import (
	"io/fs"

	usageapp "github.com/jmnote/aigauge/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var singleInstanceKey = [32]byte{
	0x61, 0x69, 0x67, 0x61, 0x75, 0x67, 0x65, 0x2d,
	0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2d,
	0x6b, 0x65, 0x79, 0x2d, 0x76, 0x31, 0x2d, 0x30,
}

type runtime struct {
	application *application.App
	window      *application.WebviewWindow
	icon        []byte
}

func Run(frontendAssets fs.FS, icon []byte) error {
	rt := &runtime{icon: icon}
	appService := usageapp.NewApp(rt.setWindowSize, rt.setAlwaysOnTop, rt.hideToTray)

	rt.application = application.New(application.Options{
		Name: "AI Gauge",
		Icon: icon,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.aigauge.app",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				rt.showWindow()
			},
		},
	})
	rt.window = rt.application.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AI Gauge",
		Width:     300,
		Height:    500,
		Frameless: true,
		Windows: application.WindowsWindow{
			NonClientRegionSupport: true,
		},
	})
	rt.configureWindow()
	rt.configureTray()
	return rt.application.Run()
}

func (rt *runtime) setWindowSize(width, height int) {
	if rt.window == nil {
		return
	}
	rt.window.SetSize(width, height)
	rt.clampWindow()
}

func (rt *runtime) setAlwaysOnTop(alwaysOnTop bool) {
	if rt.window == nil {
		return
	}
	rt.window.SetAlwaysOnTop(alwaysOnTop)
}

func (rt *runtime) showWindow() {
	if rt.window == nil {
		return
	}
	rt.window.Restore()
	rt.window.Show()
	rt.window.Focus()
}
