package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestThresholdMigration(t *testing.T) {
	fixtures := []struct {
		name     string
		settings Settings
		want     Thresholds
	}{
		{
			name: "legacy endpoints",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: true, Value: 1},
					Critical: Threshold{Enabled: true, Value: 99},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: true, Value: 5},
				Critical: Threshold{Enabled: true, Value: 100},
			},
		},
		{
			name: "rounded upper bound",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: true, Value: 42},
					Critical: Threshold{Enabled: true, Value: 98},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: true, Value: 40},
				Critical: Threshold{Enabled: true, Value: 100},
			},
		},
		{
			name: "legacy disabled zero",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: true, Value: 50},
					Critical: Threshold{Enabled: false, Value: 0},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: true, Value: 50},
				Critical: Threshold{Enabled: false, Value: 5},
			},
		},
		{
			name: "legacy enabled zero",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: true, Value: 50},
					Critical: Threshold{Enabled: true, Value: 0},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: true, Value: 50},
				Critical: Threshold{Enabled: true, Value: 5},
			},
		},
		{
			name: "current maximum",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: true, Value: 100},
					Critical: Threshold{Enabled: true, Value: 100},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: true, Value: 100},
				Critical: Threshold{Enabled: true, Value: 100},
			},
		},
		{
			name: "outside range",
			settings: Settings{
				Theme:       "dark",
				StartupMode: "tray",
				Thresholds: Thresholds{
					Warning:  Threshold{Enabled: false, Value: -10},
					Critical: Threshold{Enabled: true, Value: 150},
				},
			},
			want: Thresholds{
				Warning:  Threshold{Enabled: false, Value: 5},
				Critical: Threshold{Enabled: true, Value: 100},
			},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := json.Marshal(fixture.settings)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			store := NewFileStore(path)
			got, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.Thresholds != fixture.want {
				t.Fatalf("thresholds = %+v, want %+v", got.Thresholds, fixture.want)
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
			if persisted.Thresholds != fixture.want {
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
