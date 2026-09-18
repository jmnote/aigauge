package app

import "github.com/jmnote/aigauge/internal/config"

// Load the startup preference only for a confirmed startup activation. Manual
// launches must show the window even when the sign-in preference is tray.
func startHiddenOnLaunch(activation func() (bool, error), load func() (config.Settings, error)) (bool, error) {
	startup, err := activation()
	if err != nil || !startup {
		return false, err
	}
	settings, err := load()
	if err != nil {
		return false, err
	}
	return settings.StartupMode == StartWithWindowsInTray, nil
}
