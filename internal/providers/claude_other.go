//go:build !windows

package providers

import "path/filepath"

func claudeFallbackPath(home string) (string, bool) {
	return filepath.Join(home, ".local", "bin", "claude"), true
}
