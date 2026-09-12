package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmnote/aigauge/internal/app/fixtures"
	"github.com/jmnote/aigauge/internal/providers"
)

var AppVersion = "v0.0.0"
var ThemeOverride string

type App struct {
	onContentHeight   func(height int)
	onWindowWidth     func(width int)
	onSetAlwaysOnTop  func(alwaysOnTop bool)
	onHideToTray      func()
	onSetGlobalHotkey func(enabled bool, shortcut string) error
}

func NewApp(onContentHeight func(height int), onWindowWidth func(width int), onSetAlwaysOnTop func(alwaysOnTop bool), onHideToTray func(), hotkeyHandlers ...func(enabled bool, shortcut string) error) *App {
	app := &App{
		onContentHeight:  onContentHeight,
		onWindowWidth:    onWindowWidth,
		onSetAlwaysOnTop: onSetAlwaysOnTop,
		onHideToTray:     onHideToTray,
	}
	if len(hotkeyHandlers) > 0 {
		app.onSetGlobalHotkey = hotkeyHandlers[0]
	}
	return app
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

// Diagnose* report what can be determined about a provider without contacting
// any service. The first-run screen calls these, so opening AI Gauge on a
// machine that has never been configured makes no outbound request at all;
// confirming a connection is the separate, user-initiated GetXUsage call.
func (a *App) DiagnoseCodex() providers.Diagnosis { return providers.DiagnoseCodex() }

func (a *App) DiagnoseClaude() providers.Diagnosis { return providers.DiagnoseClaude() }

func (a *App) DiagnoseAntigravity() providers.Diagnosis { return providers.DiagnoseAntigravity() }

// GetXUsage performs the real, network-backed lookup for a provider the user
// has enabled.
func (a *App) GetCodexUsage() providers.CodexUsage { return providers.GetCodexUsage() }

func (a *App) GetAntigravityUsage() providers.AntigravityUsage {
	return providers.GetAntigravityUsage()
}

func (a *App) GetClaudeUsage() providers.ClaudeUsage { return providers.GetClaudeUsage() }

// GetSampleXUsage return the bundled fixtures that back the sample-data
// preview. They are separate RPCs rather than a mode flag on the live methods
// on purpose: the preview has to be provably incapable of reading a credential,
// running a CLI, or reaching the network, and the way to guarantee that is for
// its data to arrive through methods that contain no such code. Nothing here
// reads or writes the user's provider settings either, so opening and closing
// the preview leaves the real configuration exactly as it was.
func (a *App) GetSampleCodexUsage() providers.CodexUsage {
	usage, err := decodeSample[providers.CodexUsage](fixtures.CodexJSON)
	if err != nil {
		return providers.CodexUsage{DiagnosisFields: providers.DiagnosisFields{
			Status: providers.StatusUsageUnavailable, Message: err.Error(), Error: err.Error(),
		}}
	}
	// Codex reports its reset points as remaining seconds rather than
	// timestamps, so they are already relative to now and need no shifting.
	usage.FetchedAt = time.Now().Format(time.RFC3339)
	usage.Status = providers.StatusConnected
	return usage
}

func (a *App) GetSampleClaudeUsage() providers.ClaudeUsage {
	usage, err := decodeSample[providers.ClaudeUsage](fixtures.ClaudeJSON)
	if err != nil {
		return providers.ClaudeUsage{DiagnosisFields: providers.DiagnosisFields{
			Status: providers.StatusUsageUnavailable, Message: err.Error(), Error: err.Error(),
		}}
	}
	now := time.Now()
	capturedAt := usage.FetchedAt
	for i := range usage.Buckets {
		usage.Buckets[i].ResetTime = shiftResetTime(usage.Buckets[i].ResetTime, capturedAt, now)
	}
	usage.FetchedAt = now.Format(time.RFC3339)
	usage.Status = providers.StatusConnected
	return usage
}

func (a *App) GetSampleAntigravityUsage() providers.AntigravityUsage {
	usage, err := decodeSample[providers.AntigravityUsage](fixtures.AntigravityJSON)
	if err != nil {
		return providers.AntigravityUsage{DiagnosisFields: providers.DiagnosisFields{
			Status: providers.StatusUsageUnavailable, Message: err.Error(), Error: err.Error(),
		}}
	}
	now := time.Now()
	capturedAt := usage.FetchedAt
	for gi := range usage.Groups {
		for bi := range usage.Groups[gi].Buckets {
			bucket := &usage.Groups[gi].Buckets[bi]
			bucket.ResetTime = shiftResetTime(bucket.ResetTime, capturedAt, now)
		}
	}
	usage.FetchedAt = now.Format(time.RFC3339)
	usage.Status = providers.StatusConnected
	return usage
}

func decodeSample[T any](data []byte) (T, error) {
	var usage T
	if err := json.Unmarshal(data, &usage); err != nil {
		return usage, fmt.Errorf("could not load the bundled sample data: %v", err)
	}
	return usage, nil
}

// shiftResetTime moves a fixture's reset timestamp forward by the time elapsed
// since the fixture was captured, so the preview's countdowns read as plausibly
// live instead of having expired months ago.
func shiftResetTime(resetTime, capturedAt string, now time.Time) string {
	reset, err := time.Parse(time.RFC3339, resetTime)
	if resetTime == "" || err != nil {
		return resetTime
	}
	captured, err := time.Parse(time.RFC3339, capturedAt)
	if err != nil {
		return resetTime
	}
	return reset.Add(now.Sub(captured)).Format(time.RFC3339Nano)
}
