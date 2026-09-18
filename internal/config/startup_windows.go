//go:build windows

package config

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// MigrateLegacyStartupMode removes the preference mistakenly stored as a Run
// command by the initial startup implementation. Save to the main settings
// file before deleting so failures can be retried.
func MigrateLegacyStartupMode() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	mode, _, err := key.GetStringValue("AIGaugeStartupMode")
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return migrateLegacyStartupMode(mode, func() error {
		writable, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer writable.Close()
		err = writable.DeleteValue("AIGaugeStartupMode")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	})
}
