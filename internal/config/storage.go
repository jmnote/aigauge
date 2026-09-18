package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Store defines how Settings are persisted. It mirrors internal/auth.Store so
// the two packages that own AI Gauge's on-disk state follow the same shape:
// a small interface, a package-level default instance, and platform storage
// wired up by an init() in this package.
type Store interface {
	Load() (Settings, error)
	Save(Settings) error
}

var (
	defaultStoreMu sync.RWMutex
	defaultStore   Store
)

// SetDefaultStore configures the global store instance. Tests use this to
// substitute an in-memory store instead of touching the real settings file.
func SetDefaultStore(s Store) {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()
	defaultStore = s
}

// GetDefaultStore returns the configured global store instance.
func GetDefaultStore() Store {
	defaultStoreMu.RLock()
	defer defaultStoreMu.RUnlock()
	return defaultStore
}

// Load returns the persisted settings from the default store, or the
// defaults if no store is configured or nothing has been saved yet.
func Load() (Settings, error) {
	s := GetDefaultStore()
	if s == nil {
		return Default(), nil
	}
	return s.Load()
}

// Save persists settings to the default store. It is a no-op if no store is
// configured, matching internal/auth's behavior for an unwired store.
func Save(settings Settings) error {
	s := GetDefaultStore()
	if s == nil {
		return nil
	}
	return s.Save(settings)
}

// NewInstanceID returns a short random identifier for a new ProviderInstance.
// It is independent of the provider type so several instances of the same
// type never collide.
func NewInstanceID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// fileStore persists Settings as indented JSON at a fixed path, writing
// through a temporary file and rename so a crash mid-write never leaves a
// truncated or partially-written settings file behind.
type fileStore struct {
	mu   sync.Mutex
	path string
}

// NewFileStore returns a Store backed by a plain JSON file at path. Settings
// carry no secrets - those live in internal/auth's encrypted token store -
// so, unlike credentials, they need no platform-specific encryption.
func NewFileStore(path string) Store {
	return &fileStore{path: path}
}

func (f *fileStore) Load() (Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), err
	}

	settings := Default()
	if err := json.Unmarshal(data, &settings); err != nil {
		return Default(), err
	}
	if normalized := NormalizeThresholds(settings.Thresholds); normalized != settings.Thresholds {
		settings.Thresholds = normalized
		if err := f.saveLocked(settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func (f *fileStore) Save(settings Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saveLocked(settings)
}

func (f *fileStore) saveLocked(settings Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	SetDefaultStore(NewFileStore(filepath.Join(dir, "aigauge", "settings.json")))
}
