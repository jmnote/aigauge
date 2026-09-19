//go:build windows

package providers

import (
	"os/exec"
	"path/filepath"
	"syscall"
)

// createNewConsole (CREATE_NEW_CONSOLE) gives agy its own (hidden) console
// instead of none, so any console-app children it spawns attach to that
// hidden console rather than each flashing a fresh one of their own.
const createNewConsole = 0x00000010

func configureHiddenCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNewConsole,
	}
}

func antigravityFallbackPath(home string) (string, bool) {
	return filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe"), true
}
