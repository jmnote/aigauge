//go:build !windows

package ui

func (rt *runtime) hideToTray() {
	if rt.window != nil {
		rt.window.Hide()
	}
}
