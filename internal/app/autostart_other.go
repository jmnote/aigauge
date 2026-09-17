//go:build !windows

package app

func getStartWithWindowsState() (string, error) {
	return StartWithWindowsOff, nil
}

func setStartWithWindows(string) error {
	return nil
}
