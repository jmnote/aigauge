package ui

import "github.com/wailsapp/wails/v3/pkg/application"

func (rt *runtime) configureTray() {
	tray := rt.application.SystemTray.New()
	tray.SetLabel("AI Gauge")
	tray.SetIcon(rt.icon)
	tray.OnClick(func() {
		rt.showWindow()
	})

	menu := rt.application.NewMenu()
	menu.Add("Show").OnClick(func(_ *application.Context) {
		rt.showWindow()
	})
	menu.Add("Exit").OnClick(func(_ *application.Context) {
		rt.application.Quit()
	})
	tray.SetMenu(menu)
}
