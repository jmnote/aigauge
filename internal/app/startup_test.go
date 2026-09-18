package app

import (
	"errors"
	"testing"

	"github.com/jmnote/aigauge/internal/config"
)

type failingStartupStore struct{ config.Store }

func (f failingStartupStore) Save(config.Settings) error { return errors.New("settings write failed") }

func TestStartupChangePersistsOnlyAfterOSAccepts(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	actual := StartWithWindowsOff
	denied := errors.New("disabled by policy")
	app.SetStartWithWindowsHandlers(func() (string, error) { return actual, nil }, func(state string) error { return denied })
	if err := app.SetStartWithWindows(StartWithWindowsInTray); !errors.Is(err, denied) {
		t.Fatalf("error = %v", err)
	}
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StartupMode != "" {
		t.Fatalf("rejected mode saved: %q", settings.StartupMode)
	}
	app.SetStartWithWindowsHandlers(func() (string, error) { return actual, nil }, func(state string) error { actual = state; return nil })
	if err := app.SetStartWithWindows(StartWithWindowsInTray); err != nil {
		t.Fatal(err)
	}
	settings, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StartupMode != StartWithWindowsInTray {
		t.Fatalf("saved mode = %q", settings.StartupMode)
	}
	store := config.GetDefaultStore()
	config.SetDefaultStore(failingStartupStore{store})
	if err := app.SetStartWithWindows(StartWithWindowsShow); err == nil {
		t.Fatal("expected save error")
	}
	if actual != StartWithWindowsInTray {
		t.Fatalf("OS state not restored: %q", actual)
	}
	settings, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StartupMode != StartWithWindowsInTray {
		t.Fatal("previous mode changed after failed save")
	}
	config.SetDefaultStore(store)
	if err := app.SetStartWithWindows(StartWithWindowsOff); err != nil {
		t.Fatal(err)
	}
	settings, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StartupMode != StartWithWindowsOff || actual != StartWithWindowsOff {
		t.Fatal("Off did not update OS and settings")
	}
}
