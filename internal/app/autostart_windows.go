//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runRegistryName = `AIGauge`
)

func isStartOnBootEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()

	val, _, err := k.GetStringValue(runRegistryName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return val != "", nil
}

func setStartOnBoot(enabled bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !enabled {
		err := k.DeleteValue(runRegistryName)
		if err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	evalPath, err := filepath.EvalSymlinks(exePath)
	if err == nil {
		exePath = evalPath
	}
	exePath = filepath.Clean(exePath)

	cmd := fmt.Sprintf("\"%s\" --hidden", exePath)
	return k.SetStringValue(runRegistryName, cmd)
}
