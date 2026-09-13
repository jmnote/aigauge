//go:build ignore

// Command gen-samples fetches real usage data from Codex, Claude, and
// Antigravity using the same providers the shipped app uses, and writes it
// to hack/fixtures/samples/sample-codex.json, sample-claude.json, and
// sample-antigravity.json - the exact fixtures the live-server preview
// (hack/live-server.mjs) serves back for each provider's RPC method, with no
// conversion step in between. A separate step, hack/fixtures/gen-samples-go, then
// compiles these into internal/app/fixtures/fixtures.go for the app's
// sample-data preview (internal/app.App) - `.\build.ps1 fixtures` runs both
// this and that in sequence.
//
// Run via `.\build.ps1 fixtures-json` (or `.\build.ps1 fixtures` for the
// full pipeline) from the repository root. Because it uses the real local
// Codex/Claude session and the local `agy` CLI, the output reflects the
// developer's own account (plan tier, usage percentages, timestamps) -
// review before committing hack/fixtures/samples/sample-*.json.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmnote/aigauge/internal/providers"
)

func main() {
	samplesDir := filepath.Join("hack", "fixtures", "samples")
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gen-samples: create samples dir:", err)
		os.Exit(1)
	}

	fmt.Println("Fetching Codex usage...")
	codex := providers.GetCodexUsage()
	writeJSON(filepath.Join(samplesDir, "sample-codex.json"), codex)

	fmt.Println("Fetching Claude usage...")
	claude := providers.GetClaudeUsage()
	writeJSON(filepath.Join(samplesDir, "sample-claude.json"), claude)

	fmt.Println("Fetching Antigravity usage (this can take up to 45s)...")
	antigravity := providers.GetAntigravityUsage()
	writeJSON(filepath.Join(samplesDir, "sample-antigravity.json"), antigravity)

	fmt.Println("Wrote sample fixtures to", samplesDir)
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
		fmt.Fprintln(os.Stderr, "gen-samples: marshal", path, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-samples: write", path, err)
		os.Exit(1)
	}
}
