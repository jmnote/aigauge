//go:build windows

package providers

import (
	"path/filepath"
)

func antigravityFallbackPath(home string) (string, bool) {
	return filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe"), true
}
