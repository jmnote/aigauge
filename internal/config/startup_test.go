package config

import (
	"errors"
	"path/filepath"
	"testing"
)

type failedStartupMigrationStore struct{ Store }

func (s failedStartupMigrationStore) Save(Settings) error { return errors.New("write failed") }

func TestLegacyStartupMigration(t *testing.T) {
	for _, tc := range []struct {
		current, legacy, want string
		fail                  bool
	}{
		{"", "tray", "tray", false}, {"", "show", "show", false}, {"", "off", "off", false},
		{"off", "tray", "off", false}, {"tray", "show", "tray", false}, {"", "invalid", "", false}, {"", "tray", "", true},
	} {
		t.Run(tc.current+"/"+tc.legacy, func(t *testing.T) {
			store := NewFileStore(filepath.Join(t.TempDir(), "settings.json"))
			settings := Default()
			settings.StartupMode = tc.current
			if err := store.Save(settings); err != nil {
				t.Fatal(err)
			}
			SetDefaultStore(store)
			t.Cleanup(func() { SetDefaultStore(nil) })
			if tc.fail {
				SetDefaultStore(failedStartupMigrationStore{store})
			}
			removed := false
			err := migrateLegacyStartupMode(tc.legacy, func() error { removed = true; return nil })
			if (err != nil) != tc.fail || removed == tc.fail {
				t.Fatalf("error=%v removed=%v", err, removed)
			}
			got, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.StartupMode != tc.want {
				t.Fatalf("mode=%q want=%q", got.StartupMode, tc.want)
			}
		})
	}
}
