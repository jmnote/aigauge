//go:build windows

package providers

import "path/filepath"

func codexFallbackPath(home string) (string, bool) {
	return filepath.Join(home, "AppData", "Local", "Programs", "OpenAI", "Codex", "bin", "codex.exe"), true
}
