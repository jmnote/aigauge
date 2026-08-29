package app

import "github.com/jmnote/aigauge/internal/providers"

var AppVersion = "v0.0.0"
var ThemeOverride string

type App struct{}

func (a *App) GetVersion() string { return AppVersion }

func (a *App) GetThemeOverride() string { return ThemeOverride }

// Keep these methods on App for Wails bindings; the implementations live in providers.
func (a *App) GetCodexUsage() providers.CodexUsage {
	return providers.GetCodexUsage()
}

func (a *App) GetAntigravityUsage() providers.AntigravityUsage {
	return providers.GetAntigravityUsage()
}
