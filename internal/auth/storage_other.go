//go:build !windows

package auth

import (
	"os"
	"path/filepath"
)

func init() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "aigauge", "credentials.json")
	SetDefaultStore(NewFileStore(path))
}

// NewFileStore creates a token store backed by a 0600 file on disk.
func NewFileStore(filePath string) Store {
	return newFileStore(filePath, identity, identity)
}

// identity leaves data unchanged - non-Windows has no equivalent of
// Windows DPAPI encryption-at-rest available, so tokens are protected only by
// the file's 0600 permissions (see NewFileStore).
func identity(data []byte) ([]byte, error) {
	return data, nil
}
