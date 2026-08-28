package main

// appVersion is the single source of truth for the application version.
// Release builds can override it with: -ldflags "-X main.appVersion=1.0.0"
var appVersion = "0.1.0"

func (a *App) GetVersion() string {
	return appVersion
}
