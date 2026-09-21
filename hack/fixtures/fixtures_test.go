//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmnote/aigauge/internal/providers"
)

func TestSaveCodexUsageSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name      string
		window    string
		wantError string
	}{
		{"valid", `{"used_percent":10,"reset_after_seconds":60}`, ""},
		{"missing", `{"used_percent":10}`, "rate_limit.primary_window.reset_after_seconds is missing or null"},
		{"percent", `{"used_percent":101,"reset_after_seconds":60}`, "rate_limit.primary_window.used_percent = 101"},
		{"reset", `{"used_percent":10,"reset_after_seconds":-1}`, "rate_limit.primary_window.reset_after_seconds = -1"},
		{"type", `{"used_percent":"unexpected","reset_after_seconds":60}`, "cannot unmarshal string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"user_id":"private-user","email":"private@example.com","rate_limit":{"primary_window":` + tc.window + `,"secondary_window":{"used_percent":20,"reset_after_seconds":120}}}`)
			usage, parseErr := providers.ParseCodexUsage(raw)
			usage.Status = providers.StatusConnected
			dir := t.TempDir()
			err := saveUsageSnapshot("codex", dir, raw, usage.ToDisplay(), parseErr)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			saved, err := os.ReadFile(filepath.Join(dir, "usage_codex.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(saved) || strings.Contains(string(saved), "private-user") || strings.Contains(string(saved), "private@example.com") {
				t.Fatal("raw fixture must be valid JSON with obfuscated identifiers")
			}
			var original, snapshot map[string]json.RawMessage
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(saved, &snapshot); err != nil {
				t.Fatal(err)
			}
			var compactOriginal, compactSnapshot any
			if err := json.Unmarshal(original["rate_limit"], &compactOriginal); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(snapshot["rate_limit"], &compactSnapshot); err != nil {
				t.Fatal(err)
			}
			originalLimit, _ := json.Marshal(compactOriginal)
			savedLimit, _ := json.Marshal(compactSnapshot)
			if string(originalLimit) != string(savedLimit) {
				t.Fatal("raw usage fields changed")
			}
			_, err = os.Stat(filepath.Join(dir, "display_codex.json"))
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && !os.IsNotExist(err) {
				t.Fatalf("display fixture should not exist: %v", err)
			}
		})
	}
}

func TestSaveCodexUsageSnapshotSanitizationFailure(t *testing.T) {
	dir := t.TempDir()
	err := saveUsageSnapshot("codex", dir, []byte(`{"user_id":{},"email":"private@example.com"}`), providers.DisplayUsage{}, nil)
	if err == nil {
		t.Fatal("expected sanitization error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("must not save unsanitized response")
	}
}
