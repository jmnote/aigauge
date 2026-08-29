//go:build !windows

package providers

import "os/exec"

func configureAntigravityCommand(command *exec.Cmd) {}
