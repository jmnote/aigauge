package app

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jmnote/aigauge/internal/config"
)

func TestStartupActivationFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/startup-activation.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name    string
		Startup bool
		Mode    string
		Hidden  bool
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			loads := 0
			hidden, err := startHiddenOnLaunch(func() (bool, error) { return fixture.Startup, nil }, func() (config.Settings, error) {
				loads++
				return config.Settings{StartupMode: fixture.Mode}, nil
			})
			if err != nil || hidden != fixture.Hidden {
				t.Fatalf("hidden=%v err=%v, want %v", hidden, err, fixture.Hidden)
			}
			if !fixture.Startup && loads != 0 {
				t.Fatal("manual launch read the startup preference")
			}
		})
	}
}

func TestStartupActivationErrorsLeaveWindowVisible(t *testing.T) {
	failure := errors.New("activation unavailable")
	hidden, err := startHiddenOnLaunch(func() (bool, error) { return false, failure }, func() (config.Settings, error) {
		t.Fatal("settings read after activation failed")
		return config.Settings{}, nil
	})
	if hidden || !errors.Is(err, failure) {
		t.Fatalf("hidden=%v err=%v", hidden, err)
	}
	hidden, err = startHiddenOnLaunch(func() (bool, error) { return true, nil }, func() (config.Settings, error) {
		return config.Settings{StartupMode: StartWithWindowsInTray}, failure
	})
	if hidden || !errors.Is(err, failure) {
		t.Fatalf("hidden=%v err=%v", hidden, err)
	}
}
