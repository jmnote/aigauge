package ui

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (rt *runtime) configureWindow() {
	initialPlacement := true
	placeTopRight := func() {
		screen := rt.application.Screen.GetPrimary()
		if screen == nil {
			return
		}
		width, _ := rt.window.Size()
		rt.window.SetPosition(screen.WorkArea.X+screen.WorkArea.Width-width, screen.WorkArea.Y)
	}
	placeInitially := func() {
		if initialPlacement {
			placeTopRight()
			initialPlacement = false
		}
	}
	rt.window.OnWindowEvent(events.Windows.WindowShow, func(_ *application.WindowEvent) {
		placeInitially()
	})
	rt.application.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		placeInitially()
	})
	rt.window.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
		rt.clampWindow()
	})
}

func (rt *runtime) clampWindow() {
	screen, err := rt.window.GetScreen()
	if err != nil || screen == nil {
		return
	}
	x, y := rt.window.Position()
	width, height := rt.window.Size()
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
	if newX != x || newY != y {
		rt.window.SetPosition(newX, newY)
	}
}
