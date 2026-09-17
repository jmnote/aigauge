//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	modeKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	modeName = "AIGaugeStartupMode"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		return
	}

	args := []string{}
	key, err := registry.OpenKey(registry.CURRENT_USER, modeKey, registry.QUERY_VALUE)
	if err == nil {
		mode, _, readErr := key.GetStringValue(modeName)
		key.Close()
		if readErr == nil && strings.EqualFold(mode, "tray") {
			args = append(args, "--hidden")
		}
	}

	cmd := exec.Command(filepath.Join(filepath.Dir(executable), "aigauge.exe"), args...)
	_ = cmd.Start()
}
