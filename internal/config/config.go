// Package config persists AI Gauge's settings - provider instances (including
// their refresh intervals), thresholds, theme and hotkey - as a single JSON document owned by the
// Go backend. It replaces the previous design where the frontend kept this
// state in each window's localStorage and reconciled the two windows with a
// storage-event/Wails-event round trip: now Go is the single source of truth,
// and both windows read it through GetSettings, apply field-level updates
// through backend RPCs, and are notified of changes via a Wails event.
package config

// ProviderInstance is one user-added provider connection. Unlike the previous
// model - one fixed row per provider type - a user may add several instances
// of the same Type (for example two separate Claude accounts), so ID rather
// than Type identifies a row and is what the auth token store is keyed by.
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
