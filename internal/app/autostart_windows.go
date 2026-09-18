//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runRegistryName = "AIGauge"
	startupTaskID   = "AIGaugeStartup"
)

func getStartWithWindowsState() (string, error) {
	if isPackagedWindowsApp() {
		return getPackagedStartWithWindowsState()
	}
	return getRegistryStartWithWindowsState()
}

func getRegistryStartWithWindowsState() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return StartWithWindowsOff, nil
		}
		return "", err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(runRegistryName)
	if err != nil {
		if err == registry.ErrNotExist {
			return StartWithWindowsOff, nil
		}
		return "", err
	}
	if registryCommandStartsInTray(value) {
		return StartWithWindowsInTray, nil
	}
	return StartWithWindowsShow, nil
}

func registryCommandStartsInTray(command string) bool {
	command = strings.TrimSpace(command)
	for _, flag := range []string{"--hidden", "--tray", "--minimized"} {
		if strings.HasSuffix(command, " "+flag) {
			return true
		}
	}
	return false
}

func setStartWithWindows(state string) error {
	if isPackagedWindowsApp() {
		return setPackagedStartWithWindows(state)
	}
	return setRegistryStartWithWindows(state)
}

func setRegistryStartWithWindows(state string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if state == StartWithWindowsOff {
		err := key.DeleteValue(runRegistryName)
		if err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	command := fmt.Sprintf("\"%s\"", filepath.Clean(executable))
	if state == StartWithWindowsInTray {
		command += " --hidden"
	}
	return key.SetStringValue(runRegistryName, command)
}
