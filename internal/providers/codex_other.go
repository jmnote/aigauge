//go:build !windows

package providers

import "path/filepath"

func codexFallbackPath(home string) (string, bool) {
	return filepath.Join(home, ".local", "bin", "codex"), true
}
