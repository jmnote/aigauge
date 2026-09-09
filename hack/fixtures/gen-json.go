//go:build ignore

// Command gen-json fetches real usage data from Codex, Claude, and
// Antigravity using the same providers the shipped app uses, and writes it
// to hack/fixtures/sample-codex.json, sample-claude.json, and
// sample-antigravity.json - the exact fixtures the live-server preview
// (hack/live-server.ps1) serves back for each provider's RPC method, with no
// conversion step in between. A separate step, hack/fixtures/gen-go, then
// compiles these into internal/app/fixtures/fixtures.go for the app's
// sample-data preview (internal/app.App) - `.\build.ps1 fixtures` runs both
// this and that in sequence.
//
// Run via `.\build.ps1 fixtures-json` (or `.\build.ps1 fixtures` for the
// full pipeline) from the repository root. Because it uses the real local
// Codex/Claude session and the local `agy` CLI, the output reflects the
// developer's own account (plan tier, usage percentages, timestamps) -
// review before committing hack/fixtures/sample-*.json.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmnote/aigauge/internal/providers"
)

func main() {
	fixturesDir := filepath.Join("hack", "fixtures")
	if _, err := os.Stat(fixturesDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-json: run this from the repository root:", err)
		os.Exit(1)
	}

	fmt.Println("Fetching Codex usage...")
	codex := providers.GetCodexUsage()
	writeJSON(filepath.Join(fixturesDir, "sample-codex.json"), codex)

	fmt.Println("Fetching Claude usage...")
	claude := providers.GetClaudeUsage()
	writeJSON(filepath.Join(fixturesDir, "sample-claude.json"), claude)

	fmt.Println("Fetching Antigravity usage (this can take up to 45s)...")
	antigravity := providers.GetAntigravityUsage()
	writeJSON(filepath.Join(fixturesDir, "sample-antigravity.json"), antigravity)

	fmt.Println("Wrote sample fixtures to", fixturesDir)
	for _, failure := range []struct{ name, message string }{
		{"Codex", codex.Error},
		{"Claude", claude.Error},
		{"Antigravity", antigravity.Error},
	} {
		if failure.message != "" {
			fmt.Fprintf(os.Stderr, "warning: %s fetch failed: %s\n", failure.name, failure.message)
		}
	}
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-json: marshal", path, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-json: write", path, err)
		os.Exit(1)
	}
}
