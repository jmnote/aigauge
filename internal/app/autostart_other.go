//go:build !windows

package app

func isStartOnBootEnabled() (bool, error) {
	return false, nil
}

func setStartOnBoot(enabled bool) error {
	return nil
}
