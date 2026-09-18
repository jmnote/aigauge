package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultHasNoProvidersAndSaneDefaults(t *testing.T) {
	d := Default()
	if len(d.Providers) != 0 {
		t.Errorf("Default().Providers = %#v, want empty so onboarding drives the first Add", d.Providers)
	}
	if d.Theme != "system" {
		t.Errorf("Default() = %+v, want theme=system", d)
	}
	if !d.Thresholds.Warning.Enabled || d.Thresholds.Warning.Value != 50 {
		t.Errorf("Default().Thresholds.Warning = %+v, want enabled at 50", d.Thresholds.Warning)
	}
}

func TestValidateThresholdsRequiresCriticalAtOrBelowWarning(t *testing.T) {
	valid := Thresholds{
		Warning:  Threshold{Enabled: true, Value: 50},
		Critical: Threshold{Enabled: true, Value: 20},
	}
	if err := ValidateThresholds(valid); err != nil {
		t.Fatalf("ValidateThresholds(valid) error = %v", err)
	}

	invalid := valid
	invalid.Warning.Value = 20
	invalid.Critical.Value = 90
	if err := ValidateThresholds(invalid); err == nil {
		t.Fatal("ValidateThresholds(invalid) error = nil, want ordering error")
	}

	disabled := invalid
	disabled.Critical.Enabled = false
	if err := ValidateThresholds(disabled); err != nil {
		t.Fatalf("ValidateThresholds(disabled critical) error = %v", err)
	}
}

func TestLoadAndSaveWithoutADefaultStoreAreNoops(t *testing.T) {
	SetDefaultStore(nil)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Providers) != 0 {
		t.Errorf("Load() without a store = %+v, want the defaults", got)
	}

	if err := Save(Settings{Theme: "dark"}); err != nil {
		t.Errorf("Save() without a store error = %v, want nil (no-op)", err)
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := NewFileStore(path)
	SetDefaultStore(store)
	t.Cleanup(func() { SetDefaultStore(nil) })

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() on a missing file error = %v", err)
	}
	if len(loaded.Providers) != 0 {
		t.Errorf("Load() on a missing file = %+v, want the defaults", loaded)
	}

	want := Settings{
		Providers: []ProviderInstance{
			{ID: "abc123", Type: "claude", Label: "Claude"},
			{ID: "def456", Type: "claude", Label: "Claude #2"},
		},
		WindowWidth: 320,
		Theme:       "dark",
		Thresholds: Thresholds{
			Warning:  Threshold{Enabled: true, Value: 40},
			Critical: Threshold{Enabled: false, Value: 10},
		},
		HotkeyShortcut: "Super+Shift+`",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if len(got.Providers) != 2 || got.Providers[1].Label != "Claude #2" {
		t.Fatalf("Load() after Save() = %+v, want the two saved instances back", got)
	}
	if got.Theme != "dark" || got.HotkeyShortcut != "Super+Shift+`" {
		t.Errorf("Load() after Save() = %+v, want the saved preferences back", got)
	}
}

func TestNewInstanceIDIsUniqueAndNonEmpty(t *testing.T) {
	a, err := NewInstanceID()
	if err != nil {
		t.Fatalf("NewInstanceID() error = %v", err)
	}
	b, err := NewInstanceID()
	if err != nil {
		t.Fatalf("NewInstanceID() error = %v", err)
	}
	if a == "" || b == "" {
		t.Fatal("NewInstanceID() returned an empty id")
	}
	if a == b {
		t.Errorf("NewInstanceID() returned the same id twice: %q", a)
	}
}
