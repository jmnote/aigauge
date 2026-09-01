package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmnote/aigauge/internal/providers"
)

var AppVersion = "v0.0.0"
var ThemeOverride string

// FixturesDir, when set (via --fixtures=<dir>), makes the usage RPC methods
// below return that directory's saved sample-*.json fixtures - the same
// files hack/gensample writes and hack/live-server.ps1 serves to the browser
// preview - instead of calling the real provider APIs. It exists for fast,
// deterministic runs of the real native app when only a realistic-looking
// UI is needed and the live network round-trips aren't: hack/screenshot.ps1
// uses it so `.\build.ps1 screenshot` doesn't have to wait on (or even be
// logged into) Codex/Claude/Antigravity just to render the window.
var FixturesDir string

func loadFixture(name string, out any) error {
	data, err := os.ReadFile(filepath.Join(FixturesDir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

type App struct {
	onResize         func(width, height int)
	onSetAlwaysOnTop func(alwaysOnTop bool)
	onHideToTray     func()
}

func NewApp(onResize func(width, height int), onSetAlwaysOnTop func(alwaysOnTop bool), onHideToTray func()) *App {
	return &App{
		onResize:         onResize,
		onSetAlwaysOnTop: onSetAlwaysOnTop,
		onHideToTray:     onHideToTray,
	}
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

func (a *App) SetContentHeight(height int) {
	if a.onResize == nil {
		return
	}
	if height < 80 {
		height = 80
	}
	if height > 1600 {
		height = 1600
	}
	a.onResize(250, height)
}

// Keep these methods on App for Wails bindings; the implementations live in providers.
func (a *App) GetCodexUsage() providers.CodexUsage {
	if FixturesDir != "" {
		var usage providers.CodexUsage
		if err := loadFixture("sample-codex.json", &usage); err != nil {
			return providers.CodexUsage{Error: fmt.Sprintf("Failed to load fixture: %v", err)}
		}
		return usage
	}
	return providers.GetCodexUsage()
}

func (a *App) GetAntigravityUsage() providers.AntigravityUsage {
	if FixturesDir != "" {
		var usage providers.AntigravityUsage
		if err := loadFixture("sample-antigravity.json", &usage); err != nil {
			return providers.AntigravityUsage{Error: fmt.Sprintf("Failed to load fixture: %v", err)}
		}
		return usage
	}
	return providers.GetAntigravityUsage()
}

func (a *App) GetClaudeUsage() providers.ClaudeUsage {
	if FixturesDir != "" {
		var usage providers.ClaudeUsage
		if err := loadFixture("sample-claude.json", &usage); err != nil {
			return providers.ClaudeUsage{Error: fmt.Sprintf("Failed to load fixture: %v", err)}
		}
		return usage
	}
	return providers.GetClaudeUsage()
}
