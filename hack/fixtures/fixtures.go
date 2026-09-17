//go:build ignore

// Command fixtures captures a usage snapshot using AI Gauge's own stored
// credentials and provider logic (internal/providers), writing both:
//   - hack/fixtures/usage/usage_<provider>_*.json - the API's raw response, byte
//     for byte (Codex's user_id/email redacted)
//   - hack/fixtures/display/display_<provider>_*.json - that same response parsed
//     and converted (ParseXUsage + ToDisplay), the shape the app renders
//
// from a single API call per provider. hack/fixtures/gen-embed.go then
// embeds the latest display/ snapshot per provider into internal/app/fixtures
// for the sample-data preview.
//
// Run via `.\build.ps1 fixtures-usage <codex|claude|antigravity|all>` from
// the repository root.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jmnote/aigauge/hack/fixtures/util"
	"github.com/jmnote/aigauge/internal/config"
	"github.com/jmnote/aigauge/internal/providers"
)

var unsafeFilenameChar = regexp.MustCompile(`[^A-Za-z0-9.-]+`)

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fixtures: "+format+"\n", args...)
	os.Exit(1)
}

func sanitize(s string) string {
	s = unsafeFilenameChar.ReplaceAllString(s, "-")
	if s == "" {
		return "unknown"
	}
	return s
}

// redactCodex clears the two fields Codex's raw usage response identifies
// the account by. The other providers' raw responses carry no comparable
// per-account identifier.
func redactCodex(raw []byte) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("response is not a JSON object: %w", err)
	}
	if _, ok := obj["user_id"]; !ok {
		return nil, fmt.Errorf("response contains no user_id field to redact")
	}
	if _, ok := obj["email"]; !ok {
		return nil, fmt.Errorf("response contains no email field to redact")
	}
	obj["user_id"] = util.Obfuscate(obj["user_id"])
	obj["email"] = util.Obfuscate(obj["email"])
	return json.Marshal(obj)
}

func writeJSON(dir, filename string, data []byte, pretty bool) error {
	if pretty {
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			return fmt.Errorf("%s is not valid JSON: %w", filename, err)
		}
		data = buf.Bytes()
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println("Wrote", path)
	return nil
}

func capture(settings config.Settings, providerType, usageDir, displayDir string) error {
	id, ok := settings.FirstInstance(providerType)
	if !ok {
		return fmt.Errorf("no %s provider instance found - add and connect one in AI Gauge first", providerType)
	}

	fetchedAt := time.Now().Format(time.RFC3339)

	var raw []byte
	var display providers.DisplayUsage
	var err error
	switch providerType {
	case "codex":
		raw, err = providers.FetchCodexRawUsage(id)
		if err == nil {
			var usage providers.CodexUsage
			usage, err = providers.ParseCodexUsage(raw)
			usage.FetchedAt = fetchedAt
			usage.Status = providers.StatusConnected
			display = usage.ToDisplay()
		}
	case "claude":
		raw, err = providers.FetchClaudeRawUsage(id)
		if err == nil {
			var usage providers.ClaudeUsage
			usage, err = providers.ParseClaudeUsage(raw)
			usage.FetchedAt = fetchedAt
			usage.Status = providers.StatusConnected
			display = usage.ToDisplay()
		}
	case "antigravity":
		raw, err = providers.FetchAntigravityRawUsage(id)
		if err == nil {
			var usage providers.AntigravityUsage
			usage, err = providers.ParseAntigravityUsage(raw)
			if err != nil {
				break
			}
			usage.FetchedAt = fetchedAt
			usage.Status = providers.StatusConnected
			display = usage.ToDisplay()
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", providerType, err)
	}
	if display.Error != "" {
		return fmt.Errorf("%s: %s", providerType, display.Error)
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.json", providerType, date)
	if providerType == "codex" {
		var plan struct {
			PlanType string `json:"plan_type"`
		}
		_ = json.Unmarshal(raw, &plan)
		filename = fmt.Sprintf("codex_%s_%s.json", sanitize(plan.PlanType), date)
		if raw, err = redactCodex(raw); err != nil {
			return fmt.Errorf("codex: %w", err)
		}
	}

	if err := writeJSON(usageDir, "usage_"+filename, raw, true); err != nil {
		return err
	}
	displayJSON, err := json.MarshalIndent(display, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: marshal display form: %w", providerType, err)
	}
	return writeJSON(displayDir, "display_"+filename, displayJSON, false)
}

func main() {
	if _, err := os.Stat(filepath.Join("hack", "fixtures")); err != nil {
		fatalf("run this from the repository root: %v", err)
	}

	target := "all"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}
	valid := map[string]bool{"all": true, "codex": true, "claude": true, "antigravity": true}
	if !valid[target] {
		fatalf("usage: go run hack/fixtures/fixtures.go <codex|claude|antigravity|all>")
	}

	settings, err := config.Load()
	if err != nil {
		fatalf("load settings: %v", err)
	}

	usageDir := filepath.Join("hack", "fixtures", "usage")
	displayDir := filepath.Join("hack", "fixtures", "display")
	for _, dir := range []string{usageDir, displayDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("create %s: %v", dir, err)
		}
	}

	targets := []string{"codex", "claude", "antigravity"}
	if target != "all" {
		targets = []string{target}
	}

	failed := false
	for _, providerType := range targets {
		if err := capture(settings, providerType, usageDir, displayDir); err != nil {
			fmt.Fprintln(os.Stderr, "fixtures:", err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
