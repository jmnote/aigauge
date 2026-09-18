//go:build ignore

// Command fixtures captures a usage snapshot using AI Gauge's own stored
// credentials and provider logic (internal/providers), writing both:
//   - hack/fixtures/usage/usage_<provider>.json - the API's raw response, byte
//     for byte (Codex's user_id/email redacted)
//   - hack/fixtures/usage/display_<provider>.json - that same response parsed
//     and converted (ParseXUsage + ToDisplay), the shape the app renders
//
// from a single API call per provider. hack/fixtures/gen-embed.go then
// embeds the usage/display_ snapshot per provider into internal/app/fixtures
// for the sample-data preview.
//
// Run via `.\build.ps1 fixtures-usage <codex|claude|antigravity|all>` from
// the repository root. Use `fixtures-tokens` for Codex and Claude token samples.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmnote/aigauge/hack/fixtures/util"
	"github.com/jmnote/aigauge/internal/auth"
	"github.com/jmnote/aigauge/internal/config"
	"github.com/jmnote/aigauge/internal/providers"
)

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fixtures: "+format+"\n", args...)
	os.Exit(1)
}

// redactCodex clears the two fields Codex's raw usage response identifies
// the account by. The other providers' raw responses carry no comparable
// per-account identifier.
func redactCodex(raw []byte) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("response is not a JSON object: %w", err)
	}
	obj = util.ObfuscateFields(obj).(map[string]any)
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

func snapshotExists(usageDir, providerType string) (bool, error) {
	for _, name := range []string{"usage_" + providerType + ".json", "display_" + providerType + ".json"} {
		if _, err := os.Stat(filepath.Join(usageDir, name)); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}

func capture(settings config.Settings, providerType, usageDir string) error {
	var exists bool
	var err error
	exists, err = snapshotExists(usageDir, providerType)
	if err != nil {
		return fmt.Errorf("check %s fixture: %w", providerType, err)
	}
	if exists {
		fmt.Printf("Skipping %s: fixture already exists in %s\n", providerType, usageDir)
		return nil
	}

	id, ok := settings.FirstInstance(providerType)
	if !ok {
		return fmt.Errorf("no %s provider instance found - add and connect one in AI Gauge first", providerType)
	}

	fetchedAt := time.Now().Format(time.RFC3339)

	var raw []byte
	var display providers.DisplayUsage
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

	filename := providerType + ".json"
	if providerType == "codex" {
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
	return writeJSON(usageDir, "display_"+filename, displayJSON, false)
}

type providerTokenConfig struct {
	id                  string
	name                string
	credentialsFileName string
}

func writeJSONValue(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func captureTokens(tokensDir, target string) int {
	configs := []providerTokenConfig{
		{id: "codex", name: "Codex", credentialsFileName: "credentials-codex.json"},
		{id: "claude", name: "Claude", credentialsFileName: "credentials-claude.json"},
	}

	var selected []providerTokenConfig
	if target == "all" || target == "" {
		selected = configs
	} else {
		for _, cfg := range configs {
			if cfg.id == target {
				selected = append(selected, cfg)
				break
			}
		}
		if len(selected) == 0 {
			fmt.Fprintf(os.Stderr, "fixtures: unknown token provider %q (valid: all, codex, claude)\n", target)
			return 1
		}
	}

	var failures []string
	for _, cfg := range selected {
		outPath := filepath.Join(tokensDir, "token_"+cfg.id+".json")
		credentialsFilePath := filepath.Join(tokensDir, cfg.credentialsFileName)
		tokenExists := false
		credentialsFileExists := false
		if _, err := os.Stat(outPath); err == nil {
			tokenExists = true
			fmt.Printf("Skipping %s: token fixture already exists at %s\n", cfg.name, outPath)
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: check %s: %v\n", outPath, err)
			failures = append(failures, cfg.name)
			continue
		}
		if _, err := os.Stat(credentialsFilePath); err == nil {
			credentialsFileExists = true
			fmt.Printf("Skipping %s: credentials fixture already exists at %s\n", cfg.name, credentialsFilePath)
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: check %s: %v\n", credentialsFilePath, err)
			failures = append(failures, cfg.name)
			continue
		}
		if tokenExists && credentialsFileExists {
			continue
		}

		fmt.Printf("Getting %s token...\n", cfg.name)
		rawData, err := auth.ReadCredentialsFile(cfg.id)
		if err != nil || len(rawData) == 0 {
			failures = append(failures, cfg.name)
			continue
		}
		var parsedRaw any
		if err := json.Unmarshal(rawData, &parsedRaw); err != nil {
			failures = append(failures, cfg.name)
			continue
		}
		if !tokenExists {
			if err := writeJSONValue(outPath, util.ObfuscateFields(parsedRaw)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write %s: %v\n", outPath, err)
				failures = append(failures, cfg.name)
				continue
			}
			fmt.Printf("  Wrote raw credentials to %s\n", outPath)
		}
		if !credentialsFileExists {
			if err := writeJSONValue(credentialsFilePath, util.ObfuscateFields(parsedRaw)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write %s: %v\n", credentialsFilePath, err)
				failures = append(failures, cfg.name)
				continue
			}
			fmt.Printf("  Wrote redacted credentials to %s\n", credentialsFilePath)
		}
	}

	for _, fail := range failures {
		fmt.Fprintf(os.Stderr, "warning: %s token fetch failed: no credentials file found\n", fail)
	}
	if len(failures) > 0 {
		return 1
	}
	return 0
}

func captureUsage(usageDir, target string) int {
	valid := map[string]bool{"all": true, "codex": true, "claude": true, "antigravity": true}
	if !valid[target] {
		fatalf("usage: go run hack/fixtures/fixtures.go <codex|claude|antigravity|all>")
	}

	settings, err := config.Load()
	if err != nil {
		fatalf("load settings: %v", err)
	}

	targets := []string{"codex", "claude", "antigravity"}
	if target != "all" {
		targets = []string{target}
	}

	failed := false
	for _, providerType := range targets {
		if err := capture(settings, providerType, usageDir); err != nil {
			fmt.Fprintln(os.Stderr, "fixtures:", err)
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}

func main() {
	if _, err := os.Stat(filepath.Join("hack", "fixtures")); err != nil {
		fatalf("run this from the repository root: %v", err)
	}

	target := "all"
	mode := "usage"
	if len(os.Args) > 1 {
		if os.Args[1] == "tokens" {
			mode = "tokens"
			if len(os.Args) > 2 {
				target = strings.ToLower(strings.TrimSpace(os.Args[2]))
			}
		} else {
			target = os.Args[1]
		}
	}

	if mode == "tokens" {
		tokensDir := filepath.Join("hack", "fixtures", "tokens")
		if err := os.MkdirAll(tokensDir, 0o755); err != nil {
			fatalf("create %s: %v", tokensDir, err)
		}
		os.Exit(captureTokens(tokensDir, target))
	}

	usageDir := filepath.Join("hack", "fixtures", "usage")
	if err := os.MkdirAll(usageDir, 0o755); err != nil {
		fatalf("create %s: %v", usageDir, err)
	}
	os.Exit(captureUsage(usageDir, target))
}
