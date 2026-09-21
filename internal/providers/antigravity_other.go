//go:build !windows

package providers

func antigravityFallbackPath(_ string) (string, bool) { return "", false }
