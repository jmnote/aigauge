//go:build windows

package providers

import (
	"os/exec"
	"syscall"
)

// https://github.com/wailsapp/wails/discussions/1734#discussioncomment-3386172
func configureHiddenCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
