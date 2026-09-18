// Package config persists AI Gauge's provider instances, thresholds, theme,
// window width and hotkey as a single JSON document owned by the Go backend.
package config

// ProviderInstance is one user-added provider connection. ID identifies the
// instance and is also the key used by the auth token store.
// Presence in Settings.Providers is the only "is it shown" signal - there is
// no separate enabled/disabled flag - so removing an instance is the only
// way to hide it, in both windows at once.
type ProviderInstance struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Label           string `json:"label"`
	RefreshInterval int    `json:"refreshInterval"`

	// Pending marks an instance created by AddProviderInstance whose
	// authentication has not yet succeeded. CommitProviderInstance clears it
	// once the first usage fetch succeeds. An instance can otherwise be left
	// behind with this still true if the app is closed or crashes mid-login;
	// App.loadSettingsLocked drops any such orphan on the next startup.
	Pending bool `json:"pending,omitempty"`
}

const DefaultRefreshInterval = 180

// Threshold is one gauge warning level: whether it is active and the percent
// remaining at which it should trigger.
type Threshold struct {
	Enabled bool `json:"enabled"`
	Value   int  `json:"value"`
}

// Thresholds bundles the two gauge warning levels the settings screen exposes.
type Thresholds struct {
	Warning  Threshold `json:"warning"`
	Critical Threshold `json:"critical"`
}

// Settings is the complete set of user preferences persisted by AI Gauge.
type Settings struct {
	Providers      []ProviderInstance `json:"providers"`
	WindowWidth    int                `json:"windowWidth"`
	Theme          string             `json:"theme"`
	Thresholds     Thresholds         `json:"thresholds"`
	HotkeyShortcut string             `json:"hotkeyShortcut"`
	StartupMode    string             `json:"startupMode,omitempty"`
}

// NormalizeThresholds migrates legacy percentages to the selectable 5% steps.
// Keep Enabled unchanged, including for legacy disabled thresholds at 0%.
func NormalizeThresholds(thresholds Thresholds) Thresholds {
	for _, threshold := range []*Threshold{&thresholds.Warning, &thresholds.Critical} {
		value := max(5, min(100, threshold.Value))
		threshold.Value = ((value + 2) / 5) * 5
	}
	return thresholds
}

// Default returns the settings a fresh install starts from: no provider
// instances (the user adds their first one through onboarding) and the same
// preference defaults the frontend previously hard-coded.
func Default() Settings {
	return Settings{
		Providers:   nil,
		WindowWidth: 250,
		Theme:       "system",
		Thresholds: Thresholds{
			Warning:  Threshold{Enabled: true, Value: 50},
			Critical: Threshold{Enabled: true, Value: 20},
		},
	}
}

// FirstInstance returns the id of the first instance of providerType in
// Providers, for tools that need to act on "the" account for a type rather
// than listing every instance.
func (s Settings) FirstInstance(providerType string) (id string, ok bool) {
	for _, p := range s.Providers {
		if p.Type == providerType {
			return p.ID, true
		}
	}
	return "", false
}
