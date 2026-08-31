package app

import "testing"

func TestAppGetVersionAndThemeOverride(t *testing.T) {
	AppVersion = "v1.2.3"
	ThemeOverride = "dark"

	app := NewApp(nil, nil, nil)
	if app.GetVersion() != "v1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", app.GetVersion(), "v1.2.3")
	}
	if app.GetThemeOverride() != "dark" {
		t.Errorf("GetThemeOverride() = %q, want %q", app.GetThemeOverride(), "dark")
	}
}

func TestAppSetAlwaysOnTop(t *testing.T) {
	var onTopState bool
	app := NewApp(nil, func(top bool) {
		onTopState = top
	}, nil)

	app.SetAlwaysOnTop(true)
	if !onTopState {
		t.Error("onSetAlwaysOnTop was not called with true on SetAlwaysOnTop")
	}

	app.SetAlwaysOnTop(false)
	if onTopState {
		t.Error("onSetAlwaysOnTop was not called with false on SetAlwaysOnTop")
	}
}

func TestAppSetContentHeight(t *testing.T) {
	var capturedWidth, capturedHeight int
	onResize := func(w, h int) {
		capturedWidth = w
		capturedHeight = h
	}

	app := NewApp(onResize, nil, nil)
	app.SetContentHeight(350)
	if capturedWidth != 300 || capturedHeight != 350 {
		t.Errorf("Resize captured (%d, %d), want (300, 350)", capturedWidth, capturedHeight)
	}

	app.SetContentHeight(10) // below min
	if capturedHeight != 80 {
		t.Errorf("Height clamped = %d, want 80", capturedHeight)
	}

	app.SetContentHeight(5000) // above max
	if capturedHeight != 1600 {
		t.Errorf("Height clamped = %d, want 1600", capturedHeight)
	}
}

func TestAppHideToTray(t *testing.T) {
	called := false
	app := NewApp(nil, nil, func() {
		called = true
	})

	app.HideToTray()
	if !called {
		t.Error("onHideToTray was not called by HideToTray")
	}
}
