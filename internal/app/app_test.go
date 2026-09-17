package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jmnote/aigauge/internal/auth"
	"github.com/jmnote/aigauge/internal/config"
	"github.com/jmnote/aigauge/internal/providers"
)

func TestGetSettingsReturnsParseErrorWithoutOverwritingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	bad := []byte(`{"providers":`)
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	config.SetDefaultStore(config.NewFileStore(path))
	auth.SetDefaultStore(newMemTokenStore())
	t.Cleanup(func() {
		config.SetDefaultStore(nil)
		auth.SetDefaultStore(nil)
	})

	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	if _, err := app.GetSettings(); err == nil {
		t.Fatal("GetSettings() error = nil, want malformed settings error")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bad) {
		t.Fatalf("settings file was overwritten: got %q, want %q", got, bad)
	}
}

// memTokenStore is an in-memory auth.Store, isolating these tests from the
// real, DPAPI-backed credential store an unrelated test (or a developer's own
// manual runs of the app) may have already populated on this machine.
type memTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*auth.Token
}

func newMemTokenStore() *memTokenStore { return &memTokenStore{tokens: map[string]*auth.Token{}} }

func (m *memTokenStore) GetToken(p string) (*auth.Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokens[p], nil
}
func (m *memTokenStore) SaveToken(p string, t *auth.Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[p] = t
	return nil
}
func (m *memTokenStore) DeleteToken(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, p)
	return nil
}
func (m *memTokenStore) ListTokens() (map[string]*auth.Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*auth.Token, len(m.tokens))
	for k, v := range m.tokens {
		out[k] = v
	}
	return out, nil
}
func (m *memTokenStore) Clear() error        { m.tokens = map[string]*auth.Token{}; return nil }
func (m *memTokenStore) IsInitialized() bool { return true }

// withIsolatedStores points the auth and config packages at fresh in-memory/
// temp-file stores for the duration of a test, restoring the previous ones
// (normally the real, on-disk stores wired by internal/auth and
// internal/config's own init()) afterward.
func withIsolatedStores(t *testing.T) *memTokenStore {
	t.Helper()
	authStore := newMemTokenStore()
	auth.SetDefaultStore(authStore)
	config.SetDefaultStore(config.NewFileStore(t.TempDir() + "/settings.json"))
	t.Cleanup(func() {
		auth.SetDefaultStore(nil)
		config.SetDefaultStore(nil)
	})
	return authStore
}

func TestAppGetVersionAndThemeOverride(t *testing.T) {
	AppVersion = "v1.2.3"
	ThemeOverride = "dark"

	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	if app.GetVersion() != "v1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", app.GetVersion(), "v1.2.3")
	}
	if app.GetThemeOverride() != "dark" {
		t.Errorf("GetThemeOverride() = %q, want %q", app.GetThemeOverride(), "dark")
	}
}

func TestAppSetAlwaysOnTop(t *testing.T) {
	var onTopState bool
	app := NewApp(nil, nil, func(top bool) {
		onTopState = top
	}, nil, nil, nil, nil)

	app.SetAlwaysOnTop(true)
	if !onTopState {
		t.Error("onSetAlwaysOnTop was not called with true on SetAlwaysOnTop")
	}

	app.SetAlwaysOnTop(false)
	if onTopState {
		t.Error("onSetAlwaysOnTop was not called with false on SetAlwaysOnTop")
	}
}

func TestAppSetContentHeight(t *testing.T) {
	var capturedHeight int
	onContentHeight := func(height int) {
		capturedHeight = height
	}

	app := NewApp(onContentHeight, nil, nil, nil, nil, nil, nil)
	app.SetContentHeight(350)
	if capturedHeight != 350 {
		t.Errorf("Height captured = %d, want 350", capturedHeight)
	}

	app.SetContentHeight(10) // below min
	if capturedHeight != 80 {
		t.Errorf("Height clamped = %d, want 80", capturedHeight)
	}

	app.SetContentHeight(5000) // above max
	if capturedHeight != 1600 {
		t.Errorf("Height clamped = %d, want 1600", capturedHeight)
	}
}

func TestAppSetWindowWidth(t *testing.T) {
	var capturedWidth int
	app := NewApp(nil, func(width int) {
		capturedWidth = width
	}, nil, nil, nil, nil, nil)

	app.SetWindowWidth(320)
	if capturedWidth != 320 {
		t.Errorf("Width captured = %d, want 320", capturedWidth)
	}

	app.SetWindowWidth(100)
	if capturedWidth != 200 {
		t.Errorf("Minimum width = %d, want 200", capturedWidth)
	}

	app.SetWindowWidth(900)
	if capturedWidth != 600 {
		t.Errorf("Maximum width = %d, want 600", capturedWidth)
	}
}

func TestAppHideToTray(t *testing.T) {
	called := false
	app := NewApp(nil, nil, nil, func() {
		called = true
	}, nil, nil, nil)

	app.HideToTray()
	if !called {
		t.Error("onHideToTray was not called by HideToTray")
	}
}

func TestAppOpenSettings(t *testing.T) {
	called := false
	app := NewApp(nil, nil, nil, nil, func() {
		called = true
	}, nil, nil)

	app.OpenSettings()
	if !called {
		t.Error("onOpenSettings was not called by OpenSettings")
	}
}

func TestAppSetGlobalHotkey(t *testing.T) {
	var enabled bool
	var shortcut string
	app := NewApp(nil, nil, nil, nil, nil, nil, func(gotEnabled bool, gotShortcut string) error {
		enabled = gotEnabled
		shortcut = gotShortcut
		return nil
	})

	if err := app.SetGlobalHotkey(true, "Super+Shift+`"); err != nil {
		t.Fatalf("SetGlobalHotkey() error = %v", err)
	}
	if !enabled || shortcut != "Super+Shift+`" {
		t.Errorf("SetGlobalHotkey() forwarded (%v, %q), want (true, %q)", enabled, shortcut, "Super+Shift+`")
	}
}

func TestAppAuthMethods(t *testing.T) {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	// Test CancelAuth
	if err := app.CancelAuth(); err != nil {
		t.Errorf("CancelAuth() error = %v", err)
	}

	// Test DisconnectProvider
	if err := app.DisconnectProvider("test-provider"); err != nil {
		t.Errorf("DisconnectProvider() error = %v", err)
	}

	// Test SetBrowserLauncher
	calledLauncher := false
	app.SetBrowserLauncher(func(u string) error {
		calledLauncher = true
		return nil
	})
	_ = app.launchBrowser("http://example.com")
	if !calledLauncher {
		t.Error("launchBrowser did not call custom browserLauncher")
	}

	// Verify that unknown provider in ConnectProvider returns an error diagnosis
	diag, err := app.ConnectProvider("non-existent-provider")
	if err == nil {
		t.Error("ConnectProvider(non-existent) expected error, got nil")
	}
	if diag.Status != providers.StatusTemporaryError {
		t.Errorf("diag.Status = %q, want %q", diag.Status, providers.StatusTemporaryError)
	}

	// Verify that unknown provider in ImportProvider returns an error diagnosis
	importDiag, err := app.ImportProvider("non-existent-provider")
	if err == nil {
		t.Error("ImportProvider(non-existent) expected error, got nil")
	}
	if importDiag.Status != providers.StatusLoginRequired {
		t.Errorf("importDiag.Status = %q, want %q", importDiag.Status, providers.StatusLoginRequired)
	}
}

// TestConnectProviderChecksAntigravityLocallyRatherThanOAuth verifies
// ConnectProvider's Antigravity special case (see app.go): unlike
// Claude/Codex, Antigravity has no OAuth client of its own (see
// internal/auth/types.go), so "Connect" must run the real agy-CLI check
// directly instead of opening a browser.
func TestConnectProviderChecksAntigravityLocallyRatherThanOAuth(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	instance, err := app.AddProviderInstance("antigravity")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}

	browserLaunched := false
	app.SetBrowserLauncher(func(string) error {
		browserLaunched = true
		return nil
	})

	diag, err := app.ConnectProvider(instance.ID)
	if err != nil {
		t.Errorf("ConnectProvider() error = %v, want nil - Antigravity's check never fails with an error", err)
	}
	if browserLaunched {
		t.Error("ConnectProvider(antigravity) launched a browser, want it to check the local agy CLI instead")
	}
	// Whether agy happens to be installed on the machine running this test is
	// not the point - what matters is that the result came from the local
	// check rather than the OAuth path, which ConnectProvider can only reach
	// by first failing to find an auth.ProviderConfig for "antigravity" and
	// reporting exactly this wording.
	if strings.Contains(diag.Message, "Authentication failed") {
		t.Errorf("diag.Message = %q, want the local agy CLI check, not an OAuth failure", diag.Message)
	}
}

func TestAddThenRemoveProviderInstanceRoundTrip(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	instance, err := app.AddProviderInstance("claude")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}
	if instance.Type != "claude" || instance.Label != "Claude" {
		t.Errorf("AddProviderInstance() = %+v, want type=claude label=Claude", instance)
	}

	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if len(settings.Providers) != 1 || settings.Providers[0].ID != instance.ID {
		t.Fatalf("GetSettings() after add = %+v, want just the new instance", settings.Providers)
	}

	if err := app.RemoveProviderInstance(instance.ID); err != nil {
		t.Fatalf("RemoveProviderInstance() error = %v", err)
	}

	settings2, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after remove error = %v", err)
	}
	if len(settings2.Providers) != 0 {
		t.Fatalf("GetSettings() after remove = %+v, want none", settings2.Providers)
	}
}

func TestAddProviderInstanceLabelsSubsequentInstancesOfTheSameType(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	first, err := app.AddProviderInstance("claude")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}
	second, err := app.AddProviderInstance("claude")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}
	if first.Label != "Claude" || second.Label != "Claude #2" {
		t.Errorf("labels = (%q, %q), want (Claude, Claude #2)", first.Label, second.Label)
	}
	if first.ID == second.ID {
		t.Error("two instances of the same type got the same id")
	}
}

func TestConcurrentFieldUpdatesDoNotLoseProviderChanges(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, err := app.AddProviderInstance("claude")
		errs <- err
	}()
	go func() {
		<-start
		errs <- app.SetTheme("dark")
	}()
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	settings, err := app.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Theme != "dark" || len(settings.Providers) != 1 || settings.Providers[0].Type != "claude" {
		t.Fatalf("concurrent updates produced %+v, want dark theme and one Claude provider", settings)
	}
}

func TestCommitProviderInstanceClearsPendingFlag(t *testing.T) {
	withIsolatedStores(t)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	instance, err := app.AddProviderInstance("claude")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}
	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if !settings.Providers[0].Pending {
		t.Fatal("newly added instance Pending = false, want true before it is committed")
	}

	if err := app.CommitProviderInstance(instance.ID); err != nil {
		t.Fatalf("CommitProviderInstance() error = %v", err)
	}
	settings2, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after commit error = %v", err)
	}
	if settings2.Providers[0].Pending {
		t.Error("Pending still true after CommitProviderInstance")
	}
}

// TestOrphanedPendingInstanceIsDroppedOnNextStartup guards against a real bug
// (see ProviderInstance.Pending): AddProviderInstance persists an instance
// before it is authenticated, and cleanup of a failed add is otherwise the
// frontend's job alone. If the app is closed or crashes between
// AddProviderInstance and CommitProviderInstance, the instance - and any
// token an in-progress OAuth flow already saved for it - must not survive
// forever as an unrecoverable "Login required" card; the next process's first
// settings load should sweep it up instead.
func TestOrphanedPendingInstanceIsDroppedOnNextStartup(t *testing.T) {
	authStore := withIsolatedStores(t)
	crashed := NewApp(nil, nil, nil, nil, nil, nil, nil)

	instance, err := crashed.AddProviderInstance("claude")
	if err != nil {
		t.Fatalf("AddProviderInstance() error = %v", err)
	}
	// Simulate an OAuth flow that finished saving a token just before the app
	// was closed, so CommitProviderInstance never ran to clear Pending.
	if err := authStore.SaveToken(instance.ID, &auth.Token{AccessToken: "orphan-token"}); err != nil {
		t.Fatalf("SaveToken() error = %v", err)
	}

	// A fresh App value with its own zeroed checkedPendingCleanup, pointed at
	// the same on-disk/in-memory stores, models the next process launch.
	restarted := NewApp(nil, nil, nil, nil, nil, nil, nil)
	settings, err := restarted.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after restart error = %v", err)
	}
	if len(settings.Providers) != 0 {
		t.Fatalf("GetSettings() after restart = %+v, want the orphaned pending instance dropped", settings.Providers)
	}
	if tok, _ := authStore.GetToken(instance.ID); tok != nil {
		t.Error("orphaned instance's token was not deleted on startup cleanup")
	}
}

// TestDeletingTheLastInstanceDoesNotResurrectLegacyMigration guards against a
// real bug: loadSettings migrates leftover legacy (pre-instance) credentials
// into a provider instance the first time it sees an empty provider list, so
// that upgrading users keep their connection. Using "the list is empty" as
// that one-time trigger breaks the moment a user deletes their last
// instance - the list is empty again, but for a completely different,
// legitimate reason - so without a separate one-time guard, GetSettings
// would resurrect it (or any other type whose legacy token is still on
// disk) on its very next call, making Delete look like it silently failed.
func TestDeletingTheLastInstanceDoesNotResurrectLegacyMigration(t *testing.T) {
	authStore := withIsolatedStores(t)
	_ = authStore.SaveToken("codex", &auth.Token{AccessToken: "legacy-codex-token"})
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)

	settings, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if len(settings.Providers) != 1 || settings.Providers[0].ID != "codex" {
		t.Fatalf("GetSettings() = %+v, want the legacy codex token migrated into one instance", settings.Providers)
	}

	if err := app.RemoveProviderInstance("codex"); err != nil {
		t.Fatalf("RemoveProviderInstance() error = %v", err)
	}

	// A leftover CLI session sitting on disk is exactly the state a real
	// machine can be in - migration must not key off "the list is empty"
	// and resurrect it just because the user deleted their last instance.
	_ = authStore.SaveToken("codex", &auth.Token{AccessToken: "leftover-legacy-token"})

	settings2, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after delete error = %v", err)
	}
	if len(settings2.Providers) != 0 {
		t.Fatalf("GetSettings() after delete = %+v, want none (migration must not re-run within a process)", settings2.Providers)
	}
}
