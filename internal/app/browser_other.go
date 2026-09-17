//go:build !windows

package app

import "os/exec"

func openSystemBrowser(targetURL string) error {
	return exec.Command("xdg-open", targetURL).Start()
}
