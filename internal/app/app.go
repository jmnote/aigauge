package app

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jmnote/aigauge/internal/auth"
	"github.com/jmnote/aigauge/internal/config"
	"github.com/jmnote/aigauge/internal/providers"
)

var AppVersion = "v0.0.0"
var ThemeOverride string

// providerTypeInfo pairs a provider type id with its human-readable label and
// its usage-diagnosis call, so every place that needs "what provider types
// exist" (AddProviderInstance's validation, migrated-legacy auto-labeling,
// diagnoseConnectedInstance's dispatch) reads from this one table instead of
// each hand-maintaining its own copy of the type list.
type providerTypeInfo struct {
	id, label string
	diagnose  func(instanceID string) providers.Diagnosis
}

var providerTypeCatalog = []providerTypeInfo{
	{"codex", "Codex", func(id string) providers.Diagnosis { return providers.GetCodexUsage(id).ToDiagnosis() }},
	{"claude", "Claude", func(id string) providers.Diagnosis { return providers.GetClaudeUsage(id).ToDiagnosis() }},
	{"antigravity", "Antigravity", func(id string) providers.Diagnosis { return providers.GetAntigravityUsage(id).ToDiagnosis() }},
}

func providerTypeLabel(id string) (string, bool) {
	for _, t := range providerTypeCatalog {
		if t.id == id {
			return t.label, true
		}
	}
	return "", false
}

type App struct {
	settingsMu        sync.Mutex
	onContentHeight   func(height int)
	onSettingsHeight  func(height int)
	onWindowWidth     func(width int)
	onSetAlwaysOnTop  func(alwaysOnTop bool)
	onHideToTray      func()
	onOpenSettings    func()
	onSettingsChanged func(config.Settings)
	onSetGlobalHotkey func(enabled bool, shortcut string) error
	browserLauncher   func(url string) error

	// checkedLegacyMigration guards loadSettings' one-time legacy-credential
	// migration (see loadSettings) so it runs at most once per running
	// process rather than once per empty provider list. Settings.Providers
	// legitimately becomes empty again whenever a user deletes their last
	// instance, and that deletion is not distinguishable, by the providers
	// list alone, from a install that has never migrated - without this
	// guard, deleting the last instance would resurrect it (or any other
	// type whose legacy token is still on disk) the moment the frontend next
	// calls GetSettings.
	checkedLegacyMigration bool

	// checkedPendingCleanup guards the one-time-per-run sweep for orphaned
	// pending instances (see ProviderInstance.Pending), the same way
	// checkedLegacyMigration guards migration: it must run once at startup,
	// not on every load, since an instance genuinely mid-add is pending too
	// and must not be swept away while its own flow is still running.
	checkedPendingCleanup bool
}

// SetSettingsContentHeight updates the settings window's content-driven height.
func (a *App) SetSettingsContentHeight(height int) {
	if a.onSettingsHeight != nil {
		a.onSettingsHeight(height)
	}
}

func (a *App) SetSettingsContentHeightHandler(handler func(int)) { a.onSettingsHeight = handler }

// NewApp wires the App service to the callbacks its host (internal/ui)
// implements against the actual window/tray/hotkey APIs. onSettingsChanged is
// invoked whenever settings are persisted, so the host can push the change to
// every open window after a field-level settings update.
func NewApp(onContentHeight func(height int), onWindowWidth func(width int), onSetAlwaysOnTop func(alwaysOnTop bool), onHideToTray func(), onOpenSettings func(), onSettingsChanged func(config.Settings), hotkeyHandlers ...func(enabled bool, shortcut string) error) *App {
	app := &App{
		onContentHeight:   onContentHeight,
		onWindowWidth:     onWindowWidth,
		onSetAlwaysOnTop:  onSetAlwaysOnTop,
		onHideToTray:      onHideToTray,
		onOpenSettings:    onOpenSettings,
		onSettingsChanged: onSettingsChanged,
	}
	if len(hotkeyHandlers) > 0 {
		app.onSetGlobalHotkey = hotkeyHandlers[0]
	}
	return app
}

func (a *App) OpenSettings() {
	if a.onOpenSettings != nil {
		a.onOpenSettings()
	}
}

func (a *App) SetWindowWidth(width int) {
	if a.onWindowWidth == nil {
		return
	}
	if width < 200 {
		width = 200
	}
	if width > 600 {
		width = 600
	}
	a.onWindowWidth(width)
}

func (a *App) GetVersion() string { return AppVersion }

func (a *App) GetThemeOverride() string { return ThemeOverride }

func (a *App) SetAlwaysOnTop(alwaysOnTop bool) {
	if a.onSetAlwaysOnTop != nil {
		a.onSetAlwaysOnTop(alwaysOnTop)
	}
}

func (a *App) HideToTray() {
	if a.onHideToTray != nil {
		a.onHideToTray()
	}
}

func (a *App) SetGlobalHotkey(enabled bool, shortcut string) error {
	if a.onSetGlobalHotkey == nil {
		return nil
	}
	return a.onSetGlobalHotkey(enabled, shortcut)
}

func (a *App) SetContentHeight(height int) {
	if a.onContentHeight == nil {
		return
	}
	if height < 80 {
		height = 80
	}
	if height > 1600 {
		height = 1600
	}
	a.onContentHeight(height)
}

func (a *App) SetBrowserLauncher(fn func(string) error) {
	a.browserLauncher = fn
}

func (a *App) launchBrowser(u string) error {
	if a.browserLauncher != nil {
		return a.browserLauncher(u)
	}
	return openSystemBrowser(u)
}

// ConnectProvider initiates the browser-based OAuth flow for a provider
// instance, saves the token under the instance's id, and confirms
// connectivity by returning the provider's diagnosis.
func (a *App) ConnectProvider(instanceID string) (providers.Diagnosis, error) {
	log.Printf("[aigauge] ConnectProvider(%q)", instanceID)
	instance, err := a.resolveInstance(instanceID)
	if err != nil {
		log.Printf("[aigauge] ConnectProvider(%q): resolveInstance failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusTemporaryError,
			Message: fmt.Sprintf("Authentication failed: %v", err),
		}, err
	}

	// Antigravity holds no OAuth client of its own (see
	// internal/auth/types.go) - it goes through the locally installed agy
	// CLI instead, which manages its own sign-in. "Connect" therefore means
	// "run the real check now" rather than a browser login.
	if instance.Type == "antigravity" {
		ready := providers.EnsureAntigravityCLI()
		if ready.Status != providers.StatusConnected {
			return ready, nil
		}
		diag := diagnoseConnectedInstance(instance)
		log.Printf("[aigauge] ConnectProvider(%q): checked agy CLI, status=%s", instanceID, diag.Status)
		return diag, nil
	}

	// A manual-code provider's authorize page redirects to a page it hosts
	// itself (see ProviderConfig.ManualCode), not back to AI Gauge, so this
	// can only open the browser and hand back a status telling the frontend
	// to collect the pasted code and call SubmitAuthCode - unlike the
	// loopback flow below, it cannot block until login completes.
	if cfg, ok := auth.GetProviderConfig(instance.Type); ok && cfg.ManualCode {
		authURL, err := auth.BeginManualAuthFlow(instance.Type, instance.ID)
		if err != nil {
			log.Printf("[aigauge] ConnectProvider(%q): BeginManualAuthFlow failed: %v", instanceID, err)
			return providers.Diagnosis{
				Status:  providers.StatusTemporaryError,
				Message: fmt.Sprintf("Authentication failed: %v", err),
			}, err
		}
		if err := a.launchBrowser(authURL); err != nil {
			log.Printf("[aigauge] ConnectProvider(%q): launchBrowser failed: %v", instanceID, err)
			return providers.Diagnosis{
				Status:  providers.StatusTemporaryError,
				Message: fmt.Sprintf("Authentication failed: %v", err),
			}, err
		}
		log.Printf("[aigauge] ConnectProvider(%q): opened manual-code login, awaiting SubmitAuthCode", instanceID)
		return providers.Diagnosis{
			Status:  providers.StatusAwaitingCode,
			Message: fmt.Sprintf("Approve access in your browser, then paste the code %s shows back here.", cfg.Name),
		}, nil
	}

	ctx := context.Background()
	if _, err := auth.StartAuthFlow(ctx, instance.Type, instance.ID, a.launchBrowser); err != nil {
		log.Printf("[aigauge] ConnectProvider(%q): StartAuthFlow failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusTemporaryError,
			Message: fmt.Sprintf("Authentication failed: %v", err),
		}, err
	}

	diag := diagnoseConnectedInstance(instance)
	log.Printf("[aigauge] ConnectProvider(%q): connected, status=%s", instanceID, diag.Status)
	return diag, nil
}

// SubmitAuthCode completes a manual-code provider's login (see
// ProviderConfig.ManualCode and ConnectProvider) using the code the user
// copied from the provider's own redirect page and pasted back into AI Gauge.
func (a *App) SubmitAuthCode(instanceID, code string) (providers.Diagnosis, error) {
	log.Printf("[aigauge] SubmitAuthCode(%q)", instanceID)
	instance, err := a.resolveInstance(instanceID)
	if err != nil {
		log.Printf("[aigauge] SubmitAuthCode(%q): resolveInstance failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusTemporaryError,
			Message: fmt.Sprintf("Authentication failed: %v", err),
		}, err
	}

	if _, err := auth.CompleteManualAuthFlow(instance.ID, code); err != nil {
		log.Printf("[aigauge] SubmitAuthCode(%q): CompleteManualAuthFlow failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusLoginRequired,
			Message: fmt.Sprintf("Authentication failed: %v", err),
		}, err
	}

	diag := diagnoseConnectedInstance(instance)
	log.Printf("[aigauge] SubmitAuthCode(%q): connected, status=%s", instanceID, diag.Status)
	return diag, nil
}

// ImportProvider explicitly imports existing local session credentials into
// AI Gauge's secure store, under the given provider instance's id.
func (a *App) ImportProvider(instanceID string) (providers.Diagnosis, error) {
	log.Printf("[aigauge] ImportProvider(%q)", instanceID)
	instance, err := a.resolveInstance(instanceID)
	if err != nil {
		log.Printf("[aigauge] ImportProvider(%q): resolveInstance failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusLoginRequired,
			Message: fmt.Sprintf("Import failed: %v", err),
		}, err
	}

	if _, err := auth.ImportLegacy(instance.Type, instance.ID); err != nil {
		log.Printf("[aigauge] ImportProvider(%q): ImportLegacy failed: %v", instanceID, err)
		return providers.Diagnosis{
			Status:  providers.StatusLoginRequired,
			Message: fmt.Sprintf("Import failed: %v", err),
		}, err
	}

	diag := diagnoseConnectedInstance(instance)
	log.Printf("[aigauge] ImportProvider(%q): connected, status=%s", instanceID, diag.Status)
	return diag, nil
}

// diagnoseConnectedInstance confirms connectivity for an instance whose OAuth
// or import flow just succeeded (or, for Antigravity, whose "Connect" is
// itself the check - see ConnectProvider), by making the same real usage
// request the card itself would make.
func diagnoseConnectedInstance(instance config.ProviderInstance) providers.Diagnosis {
	for _, t := range providerTypeCatalog {
		if t.id == instance.Type {
			return t.diagnose(instance.ID)
		}
	}
	return providers.Diagnosis{Status: providers.StatusConnected}
}

// CancelAuth cancels the OAuth authentication flow for instanceID.
func (a *App) CancelAuth(instanceID string) error {
	auth.CancelAuthFlowAndWait(instanceID)
	auth.CancelManualAuthFlow(instanceID)
	return nil
}

// Diagnose* report what can be determined about the provider instance whose
// token is stored under instanceID, without contacting any service. The
// first-run screen calls these, so opening AI Gauge on a machine that has
// never been configured makes no outbound request at all; confirming a
// connection is the separate, user-initiated GetXUsage call.
func (a *App) DiagnoseCodex(instanceID string) providers.Diagnosis {
	return providers.DiagnoseCodex(instanceID)
}

func (a *App) DiagnoseClaude(instanceID string) providers.Diagnosis {
	return providers.DiagnoseClaude(instanceID)
}

func (a *App) DiagnoseAntigravity(instanceID string) providers.Diagnosis {
	return providers.DiagnoseAntigravity(instanceID)
}

// GetXUsage performs the real, network-backed lookup for the provider
// instance whose token is stored under instanceID, converted to the common
// DisplayUsage shape the frontend's single renderer expects.
func (a *App) GetCodexUsage(instanceID string) providers.DisplayUsage {
	return providers.GetCodexUsage(instanceID).ToDisplay()
}

func (a *App) GetAntigravityUsage(instanceID string) providers.DisplayUsage {
	return providers.GetAntigravityUsage(instanceID).ToDisplay()
}

func (a *App) GetClaudeUsage(instanceID string) providers.DisplayUsage {
	return providers.GetClaudeUsage(instanceID).ToDisplay()
}

// ---------------------------------------------------------------------------
// Settings & provider instances
//
// Settings (the provider instance list/order/enabled state, theme, refresh
// interval, thresholds and hotkey) are persisted by internal/config rather
// than by each frontend window's localStorage, so the main window and the
// settings window never need to reconcile two copies of the same state: they
// read it via GetSettings, write individual fields through the Set* methods,
// and are notified of changes made by the other window through
// onSettingsChanged.
// ---------------------------------------------------------------------------

// GetSettings returns the persisted settings, migrating a pre-instance
// installation's credentials into provider instances first if needed.
func (a *App) GetSettings() (config.Settings, error) {
	return a.loadSettings()
}

// updateSettings serializes every read-modify-write settings operation. The
// main and settings windows can issue RPCs concurrently, so accepting a whole
// settings snapshot from either window would let the last stale snapshot
// silently undo an unrelated change made by the other one.
func (a *App) updateSettings(update func(*config.Settings) error) error {
	a.settingsMu.Lock()
	settings, err := a.loadSettingsLocked()
	if err == nil {
		err = update(&settings)
	}
	if err == nil {
		err = config.Save(settings)
	}
	a.settingsMu.Unlock()
	if err != nil {
		return err
	}
	a.notifySettingsChanged(settings)
	return nil
}

func (a *App) SetTheme(theme string) error {
	if theme != "light" && theme != "dark" && theme != "system" {
		return fmt.Errorf("invalid theme %q", theme)
	}
	return a.updateSettings(func(settings *config.Settings) error {
		settings.Theme = theme
		return nil
	})
}

func (a *App) SetSavedWindowWidth(width int) error {
	if width < 200 || width > 600 {
		return fmt.Errorf("window width %d is outside the supported range", width)
	}
	return a.updateSettings(func(settings *config.Settings) error {
		settings.WindowWidth = width
		return nil
	})
}

func (a *App) SetThresholds(thresholds config.Thresholds) error {
	if thresholds.Warning.Value < 1 || thresholds.Warning.Value > 100 ||
		thresholds.Critical.Value < 0 || thresholds.Critical.Value > 99 {
		return fmt.Errorf("invalid warning/critical thresholds")
	}
	return a.updateSettings(func(settings *config.Settings) error {
		if thresholds.Warning.Enabled && thresholds.Critical.Enabled && thresholds.Critical.Value >= thresholds.Warning.Value {
			thresholds.Critical.Value = thresholds.Warning.Value - 1
		}
		settings.Thresholds = thresholds
		return nil
	})
}

func (a *App) SetHotkeyShortcut(shortcut string) error {
	if shortcut != "" && shortcut != "Ctrl+Shift+G" && shortcut != "Ctrl+Shift+Q" && shortcut != "Ctrl+Shift+E" {
		return fmt.Errorf("unsupported global hotkey %q", shortcut)
	}
	return a.updateSettings(func(settings *config.Settings) error {
		settings.HotkeyShortcut = shortcut
		return nil
	})
}

func (a *App) SetProviderRefreshInterval(instanceID string, interval int) error {
	valid := interval == 60 || interval == 180 || interval == 300 || interval == 600 || interval == 1800 || interval == 3600
	if !valid {
		return fmt.Errorf("invalid refresh interval %d", interval)
	}
	return a.updateSettings(func(settings *config.Settings) error {
		for i := range settings.Providers {
			if settings.Providers[i].ID == instanceID {
				settings.Providers[i].RefreshInterval = interval
				return nil
			}
		}
		return fmt.Errorf("unknown provider instance %q", instanceID)
	})
}

func (a *App) SetProviderOrder(instanceIDs []string) error {
	return a.updateSettings(func(settings *config.Settings) error {
		if len(instanceIDs) != len(settings.Providers) {
			return fmt.Errorf("provider order does not contain every instance")
		}
		byID := make(map[string]config.ProviderInstance, len(settings.Providers))
		for _, instance := range settings.Providers {
			byID[instance.ID] = instance
		}
		reordered := make([]config.ProviderInstance, 0, len(instanceIDs))
		for _, id := range instanceIDs {
			instance, ok := byID[id]
			if !ok {
				return fmt.Errorf("unknown or duplicate provider instance %q", id)
			}
			reordered = append(reordered, instance)
			delete(byID, id)
		}
		settings.Providers = reordered
		return nil
	})
}

// AddProviderInstance creates a new provider instance of the given type
// (codex, claude or antigravity), auto-labeling it to disambiguate from any
// existing instances of the same type, and persists it. Other windows are
// notified only after CommitProviderInstance is called.
func (a *App) AddProviderInstance(providerType string) (config.ProviderInstance, error) {
	log.Printf("[aigauge] AddProviderInstance(%q)", providerType)
	typeLabel, ok := providerTypeLabel(providerType)
	if !ok {
		log.Printf("[aigauge] AddProviderInstance(%q): unknown type", providerType)
		return config.ProviderInstance{}, fmt.Errorf("unknown provider type %q", providerType)
	}

	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	settings, err := a.loadSettingsLocked()
	if err != nil {
		return config.ProviderInstance{}, err
	}
	if providerType == "antigravity" {
		for _, p := range settings.Providers {
			if p.Type == providerType {
				return config.ProviderInstance{}, fmt.Errorf("only one Antigravity instance is supported")
			}
		}
	}

	id, err := config.NewInstanceID()
	if err != nil {
		return config.ProviderInstance{}, err
	}

	usedLabels := make(map[string]bool, len(settings.Providers))
	for _, p := range settings.Providers {
		if p.Type == providerType {
			usedLabels[p.Label] = true
		}
	}
	label := typeLabel
	for n := 2; usedLabels[label]; n++ {
		label = fmt.Sprintf("%s #%d", typeLabel, n)
	}

	// The card this instance gets shows whatever DiagnoseX/GetXUsage's real
	// status is (login_required until a first successful connect, same as
	// any later re-auth) - there's no separate enabled flag to flip once
	// that first connect succeeds. Pending starts true so an instance left
	// behind by a crash or force-quit before CommitProviderInstance is swept
	// up on the next startup instead of persisting as an unrecoverable orphan.
	instance := config.ProviderInstance{ID: id, Type: providerType, Label: label, RefreshInterval: config.DefaultRefreshInterval, Pending: true}
	settings.Providers = append(settings.Providers, instance)
	if err := config.Save(settings); err != nil {
		log.Printf("[aigauge] AddProviderInstance(%q): config.Save failed: %v", providerType, err)
		return config.ProviderInstance{}, err
	}
	log.Printf("[aigauge] AddProviderInstance(%q): created id=%q label=%q, now %d provider(s)", providerType, instance.ID, instance.Label, len(settings.Providers))
	return instance, nil
}

// CommitProviderInstance publishes a successfully authenticated provider to
// other windows after its initial usage fetch succeeds, clearing the Pending
// flag AddProviderInstance set so the instance survives the next startup's
// orphan cleanup (see ProviderInstance.Pending).
func (a *App) CommitProviderInstance(instanceID string) error {
	return a.updateSettings(func(settings *config.Settings) error {
		for i := range settings.Providers {
			if settings.Providers[i].ID == instanceID {
				settings.Providers[i].Pending = false
				return nil
			}
		}
		return fmt.Errorf("unknown provider instance %q", instanceID)
	})
}

// CleanupPendingProviderInstances cancels and removes provider instances that
// were created by an add flow but never committed.
func (a *App) CleanupPendingProviderInstances() error {
	a.settingsMu.Lock()
	settings, err := a.loadSettingsLocked()
	if err != nil {
		a.settingsMu.Unlock()
		return err
	}
	kept := make([]config.ProviderInstance, 0, len(settings.Providers))
	changed := false
	for _, p := range settings.Providers {
		if !p.Pending {
			kept = append(kept, p)
			continue
		}
		auth.CancelManualAuthFlow(p.ID)
		auth.CancelAuthFlowAndWait(p.ID)
		tok, tokenErr := auth.GetToken(p.ID)
		if tokenErr == nil && tok != nil && tok.AccessToken != "" {
			// Authentication may have completed while the usage check was still
			// in flight. Preserve that completed login when the settings window
			// closes instead of deleting it in the small commit race window.
			p.Pending = false
			kept = append(kept, p)
			changed = true
			continue
		}
		if tokenErr != nil {
			// Do not destroy a pending record when the credential store itself
			// cannot be read; startup cleanup can retry this safely.
			kept = append(kept, p)
			continue
		}
		if err := auth.DeleteToken(p.ID); err != nil {
			a.settingsMu.Unlock()
			return err
		}
		changed = true
	}
	if changed {
		settings.Providers = kept
		if err := config.Save(settings); err != nil {
			a.settingsMu.Unlock()
			return err
		}
	}
	a.settingsMu.Unlock()
	if changed {
		a.notifySettingsChanged(settings)
	}
	return nil
}

// RemoveProviderInstance disconnects a provider instance's stored credentials
// and removes it from the provider list. Unlike disabling an instance, this
// cannot be undone from the settings screen - reusing the provider again
// means adding a new instance and re-authenticating.
func (a *App) RemoveProviderInstance(instanceID string) error {
	log.Printf("[aigauge] RemoveProviderInstance(%q)", instanceID)
	// A Claude manual-code flow keeps its PKCE verifier/state keyed by the
	// instance id until the code is submitted. Removing the instance must also
	// discard that pending authentication state.
	auth.CancelManualAuthFlow(instanceID)
	auth.CancelAuthFlowAndWait(instanceID)
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	settings, err := a.loadSettingsLocked()
	if err != nil {
		log.Printf("[aigauge] RemoveProviderInstance(%q): loadSettings failed: %v", instanceID, err)
		return err
	}
	log.Printf("[aigauge] RemoveProviderInstance(%q): %d provider(s) before removal: %+v", instanceID, len(settings.Providers), settings.Providers)

	kept := settings.Providers[:0]
	found := false
	for _, p := range settings.Providers {
		if p.ID == instanceID {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		log.Printf("[aigauge] RemoveProviderInstance(%q): not found among current providers", instanceID)
		return fmt.Errorf("unknown provider instance %q", instanceID)
	}
	settings.Providers = kept

	if err := auth.DeleteToken(instanceID); err != nil {
		log.Printf("[aigauge] RemoveProviderInstance(%q): DeleteToken failed: %v", instanceID, err)
		return err
	}
	if err := config.Save(settings); err != nil {
		log.Printf("[aigauge] RemoveProviderInstance(%q): config.Save failed: %v", instanceID, err)
		return err
	}
	a.notifySettingsChanged(settings)
	log.Printf("[aigauge] RemoveProviderInstance(%q): removed, %d provider(s) remain", instanceID, len(settings.Providers))
	return nil
}

func (a *App) notifySettingsChanged(settings config.Settings) {
	if a.onSettingsChanged != nil {
		a.onSettingsChanged(settings)
	}
}

// resolveInstance looks up a provider instance by id in the persisted settings.
func (a *App) resolveInstance(instanceID string) (config.ProviderInstance, error) {
	settings, err := a.loadSettings()
	if err != nil {
		return config.ProviderInstance{}, err
	}
	for _, p := range settings.Providers {
		if p.ID == instanceID {
			return p, nil
		}
	}
	return config.ProviderInstance{}, fmt.Errorf("unknown provider instance %q", instanceID)
}

// loadSettings reads the persisted settings and, the first time settings are
// loaded with no provider instances configured, migrates any credentials left
// over from a pre-instance install into instances. It reuses the provider
// type string itself ("codex", "claude", "antigravity") as the migrated
// instance's id, so the token already stored under that same key is picked
// up with no changes to the auth store at all.
func (a *App) loadSettings() (config.Settings, error) {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.loadSettingsLocked()
}

func (a *App) loadSettingsLocked() (config.Settings, error) {
	settings, err := config.Load()
	if err != nil {
		return settings, err
	}
	if !a.checkedPendingCleanup {
		a.checkedPendingCleanup = true
		settings = a.dropOrphanedPendingInstances(settings)
	}
	if len(settings.Providers) > 0 || a.checkedLegacyMigration {
		return settings, nil
	}
	a.checkedLegacyMigration = true
	log.Print("[aigauge] loadSettings: no providers yet, checking for legacy stored tokens to migrate (once per run)")

	tokens, err := auth.ListTokens()
	if err != nil || len(tokens) == 0 {
		log.Printf("[aigauge] loadSettings: no legacy tokens found (err=%v)", err)
		return settings, nil
	}

	for _, t := range providerTypeCatalog {
		tok := tokens[t.id]
		if tok == nil || tok.AccessToken == "" {
			continue
		}
		settings.Providers = append(settings.Providers, config.ProviderInstance{
			ID: t.id, Type: t.id, Label: t.label, RefreshInterval: config.DefaultRefreshInterval,
		})
	}
	if len(settings.Providers) > 0 {
		log.Printf("[aigauge] loadSettings: migrated legacy credentials into %d instance(s): %+v", len(settings.Providers), settings.Providers)
		if err := config.Save(settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

// dropOrphanedPendingInstances removes any provider instance still marked
// Pending (see ProviderInstance.Pending) and its (possibly already-saved)
// token, so an add flow interrupted by a crash or force-quit in a previous
// run does not persist forever as a "Login required" card with no recovery
// path other than manually deleting it.
func (a *App) dropOrphanedPendingInstances(settings config.Settings) config.Settings {
	kept := settings.Providers[:0]
	removed := 0
	for _, p := range settings.Providers {
		if !p.Pending {
			kept = append(kept, p)
			continue
		}
		removed++
		log.Printf("[aigauge] loadSettings: dropping orphaned pending instance id=%q type=%q (left over from an interrupted add)", p.ID, p.Type)
		if err := auth.DeleteToken(p.ID); err != nil {
			log.Printf("[aigauge] loadSettings: DeleteToken(%q) during pending cleanup failed: %v", p.ID, err)
		}
	}
	settings.Providers = kept
	if removed > 0 {
		if err := config.Save(settings); err != nil {
			log.Printf("[aigauge] loadSettings: failed to persist pending-instance cleanup: %v", err)
		}
	}
	return settings
}
