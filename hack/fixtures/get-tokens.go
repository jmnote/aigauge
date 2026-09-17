//go:build ignore

// Command get-tokens retrieves token samples from Codex and Claude and
// writes them to hack/fixtures/tokens/. Antigravity is not covered - AI Gauge
// holds no OAuth token of its own for it, since it goes through the locally
// installed agy CLI instead (see internal/providers/antigravity.go).
//
// Run via `.\build.ps1 fixtures-tokens`
// from the repository root.
// Accepts an optional provider argument: all (default), codex, claude.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmnote/aigauge/hack/fixtures/util"
)

type providerTokenConfig struct {
	id          string
	name        string
	legacyRelFn func(home string) string
}

func main() {
	if _, err := os.Stat(filepath.Join("hack", "fixtures")); err != nil {
		fmt.Fprintln(os.Stderr, "get-tokens: run this from the repository root:", err)
		os.Exit(1)
	}

	tokensDir := filepath.Join("hack", "fixtures", "tokens")
	if err := os.MkdirAll(tokensDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "get-tokens: create tokens dir:", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "get-tokens: determine user home dir:", err)
		os.Exit(1)
	}

	configs := []providerTokenConfig{
		{
			id:   "codex",
			name: "Codex",
			legacyRelFn: func(h string) string {
				return filepath.Join(h, ".codex", "auth.json")
			},
		},
		{
			id:   "claude",
			name: "Claude",
			legacyRelFn: func(h string) string {
				return filepath.Join(h, ".claude", ".credentials.json")
			},
		},
	}

	target := "all"
	if len(os.Args) > 1 {
		target = strings.ToLower(strings.TrimSpace(os.Args[1]))
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
			fmt.Fprintf(os.Stderr, "get-tokens: unknown provider %q (valid: all, codex, claude)\n", target)
			os.Exit(1)
		}
	}

	var failures []string
	for _, cfg := range selected {
		fmt.Printf("Getting %s token...\n", cfg.name)
		rawPath := cfg.legacyRelFn(home)
		hasRaw := false

		// Raw CLI credential file
		if rawData, err := os.ReadFile(rawPath); err == nil && len(rawData) > 0 {
			var parsedRaw any
			if err := json.Unmarshal(rawData, &parsedRaw); err == nil {
				filename := fmt.Sprintf("token_%s_%s.json", cfg.id, time.Now().Format("2006-01-02"))
				if cfg.id == "claude" {
					filename = fmt.Sprintf("token_claude_%s_%s.json", claudeSubscriptionType(parsedRaw), time.Now().Format("2006-01-02"))
				}
				parsedRaw = obfuscateRaw(cfg.id, parsedRaw)
				outPath := filepath.Join(tokensDir, filename)
				if err := writeJSON(outPath, parsedRaw); err == nil {
					fmt.Printf("  Wrote raw credentials to %s\n", outPath)
					hasRaw = true
				} else {
					fmt.Fprintf(os.Stderr, "  warning: write %s: %v\n", outPath, err)
				}
			}
		}

		if !hasRaw {
			failures = append(failures, cfg.name)
		}
	}

	fmt.Println("Wrote token fixtures to", tokensDir)
	for _, fail := range failures {
		fmt.Fprintf(os.Stderr, "warning: %s token fetch failed: no legacy credentials found\n", fail)
	}
}

func claudeSubscriptionType(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return "unknown"
	}
	creds, ok := m["claudeAiOauth"].(map[string]any)
	if !ok {
		return "unknown"
	}
	subscriptionType, ok := creds["subscriptionType"].(string)
	if !ok {
		return "unknown"
	}
	subscriptionType = strings.ToLower(strings.TrimSpace(subscriptionType))
	if subscriptionType == "" {
		return "unknown"
	}
	for _, r := range subscriptionType {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return "unknown"
		}
	}
	return subscriptionType
}

func obfuscateRaw(providerID string, v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	switch providerID {
	case "codex":
		if tokens, ok := m["tokens"].(map[string]any); ok {
			for _, key := range []string{"access_token", "account_id", "id_token", "refresh_token"} {
				if val, exists := tokens[key]; exists && val != nil {
					tokens[key] = util.Obfuscate(val)
				}
			}
		}
	case "claude":
		if creds, ok := m["claudeAiOauth"].(map[string]any); ok {
			for _, key := range []string{"accessToken", "refreshToken"} {
				if val, exists := creds[key]; exists && val != nil {
					creds[key] = util.Obfuscate(val)
				}
			}
		}
		if val, exists := m["organizationUuid"]; exists && val != nil {
			m["organizationUuid"] = util.Obfuscate(val)
		}
	}
	return m
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
