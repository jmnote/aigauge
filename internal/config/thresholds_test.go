package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestThresholdMigrationFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/threshold-migration.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name     string
		Settings json.RawMessage
		Expected Thresholds
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, fixture.Settings, 0600); err != nil {
				t.Fatal(err)
			}
			store := NewFileStore(path)
			got, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.Thresholds != fixture.Expected {
				t.Fatalf("thresholds = %+v, want %+v", got.Thresholds, fixture.Expected)
			}
			if got.Theme != "dark" || got.StartupMode != "tray" {
				t.Fatalf("unrelated settings changed: %+v", got)
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var persisted Settings
			if err := json.Unmarshal(saved, &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Thresholds != fixture.Expected {
				t.Fatalf("migration not persisted: %+v", persisted.Thresholds)
			}
			if _, err := store.Load(); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != string(saved) {
				t.Fatal("second load rewrote settings")
			}
		})
	}
}
