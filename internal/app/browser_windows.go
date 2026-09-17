//go:build windows

package app

import "os/exec"

func openSystemBrowser(targetURL string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL).Start()
}
