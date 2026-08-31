//go:build windows

package providers

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

// Run the non-interactive CLI without creating a console window.
// This may also suppress the console-window behavior described in agy issue #508:
// https://github.com/google-antigravity/antigravity-cli/issues/508
const createNoWindow = 0x08000000

func configureAntigravityCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

func antigravityFallbackPath(home string) (string, bool) {
	return filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe"), true
}
