//go:build !windows

package providers

import "os/exec"

func configureHiddenCommand(command *exec.Cmd) {}
