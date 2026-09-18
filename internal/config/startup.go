package config

// Keep a newer preference when removing an old Run entry. The deletion is
// deferred until persistence succeeds so a failed migration can be retried.
func migrateLegacyStartupMode(mode string, remove func() error) error {
	settings, err := Load()
	if err != nil {
		return err
	}
	if settings.StartupMode == "" && (mode == "off" || mode == "show" || mode == "tray") {
		settings.StartupMode = mode
		if err := Save(settings); err != nil {
			return err
		}
	}
	return remove()
}
