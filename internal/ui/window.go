package ui

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (rt *runtime) configureWindow() {
	rt.window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		rt.application.Quit()
	})

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
	// Wails' WM_EXITSIZEMOVE handler (as of v3.0.0-beta.15) picks WindowEndMove
	// vs WindowEndResize by checking whether the left mouse button is still
	// down - but by the time WM_EXITSIZEMOVE fires, the button that ended the
	// drag has already been released, so it always reports WindowEndResize
	// instead. This window has DisableResize set, so a resize can never
	// actually happen here; either event only ever means "a drag just ended",
	// so handle both rather than depend on which one the framework picks.
	onDragEnd := func(_ *application.WindowEvent) {
		rt.clampWindow()
	}
	rt.window.OnWindowEvent(events.Windows.WindowEndMove, onDragEnd)
	rt.window.OnWindowEvent(events.Windows.WindowEndResize, onDragEnd)
}

// edgeSnapThreshold is how close (in pixels) the window edge has to be to a
// screen edge, after a drag ends, before it snaps flush to that edge instead
// of being left with a small gap.
const edgeSnapThreshold = 20

func (rt *runtime) clampWindow() {
	screen, err := rt.window.GetScreen()
	if err != nil || screen == nil {
		return
	}
	x, y := rt.window.Position()
	width, height := rt.window.Size()
	work := screen.WorkArea
	newX := clampAxis(x, width, work.X, work.Width)
	newY := clampAxis(y, height, work.Y, work.Height)
	if newX != x || newY != y {
		rt.window.SetPosition(newX, newY)
	}
}

// clampAxis keeps a window's position within [origin, origin+extent] along
// one axis (X or Y), magnet-snapping it flush to whichever screen edge it's
// already within edgeSnapThreshold pixels of. Called once per axis, this
// covers all four screen edges: the low/high checks handle left+top and
// right+bottom respectively, depending on which axis is passed in.
func clampAxis(pos, size, origin, extent int) int {
	if size >= extent {
		return origin
	}
	if pos < origin {
		return origin
	}
	if pos+size > origin+extent {
		return origin + extent - size
	}
	if pos-origin <= edgeSnapThreshold {
		return origin
	}
	if (origin+extent)-(pos+size) <= edgeSnapThreshold {
		return origin + extent - size
	}
	return pos
}
