package app

import "github.com/jmnote/aigauge/internal/providers"

var AppVersion = "v0.0.0"
var ThemeOverride string

type App struct {
	onResize         func(width, height int)
	onSetAlwaysOnTop func(alwaysOnTop bool)
	onHideToTray     func()
}

func NewApp(onResize func(width, height int), onSetAlwaysOnTop func(alwaysOnTop bool), onHideToTray func()) *App {
	return &App{
		onResize:         onResize,
		onSetAlwaysOnTop: onSetAlwaysOnTop,
		onHideToTray:     onHideToTray,
	}
}

func (a *App) GetVersion() string { return AppVersion }

func (a *App) GetThemeOverride() string { return ThemeOverride }

func (a *App) SetAlwaysOnTop(alwaysOnTop bool) {
	if a.onSetAlwaysOnTop != nil {
		a.onSetAlwaysOnTop(alwaysOnTop)
	}
}

func (a *App) HideToTray() {
	if a.onHideToTray != nil {
		a.onHideToTray()
	}
}

func (a *App) SetContentHeight(height int) {
	if a.onResize == nil {
		return
	}
	if height < 80 {
		height = 80
	}
	if height > 1600 {
		height = 1600
	}
	a.onResize(300, height)
}

// Keep these methods on App for Wails bindings; the implementations live in providers.
func (a *App) GetCodexUsage() providers.CodexUsage {
	return providers.GetCodexUsage()
}

func (a *App) GetAntigravityUsage() providers.AntigravityUsage {
	return providers.GetAntigravityUsage()
}

func (a *App) GetClaudeUsage() providers.ClaudeUsage {
	return providers.GetClaudeUsage()
}
