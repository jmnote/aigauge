//go:build !windows

package app

// StartHiddenOnLaunch reports whether OS startup activation requests the tray.
func StartHiddenOnLaunch() (bool, error) { return false, nil }
