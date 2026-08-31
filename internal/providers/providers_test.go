package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func TestParseCodexUsage(t *testing.T) {
	usage, err := parseCodexUsage(readFixture(t, "codex-usage.json"))
	if err != nil {
		t.Fatalf("parseCodexUsage() error = %v", err)
	}
	if usage.Plan != "pro" {
		t.Errorf("Plan = %q, want %q", usage.Plan, "pro")
	}
	if usage.FiveHour != 37.5 || usage.SevenDay != 62.25 {
		t.Errorf("usage percentages = (%v, %v), want (37.5, 62.25)", usage.FiveHour, usage.SevenDay)
	}
	if usage.FiveHourIn != 3600 || usage.SevenDayIn != 86400 {
		t.Errorf("reset seconds = (%d, %d), want (3600, 86400)", usage.FiveHourIn, usage.SevenDayIn)
	}
}

func TestParseAntigravityUsage(t *testing.T) {
	usage, err := parseAntigravityUsage(readFixture(t, "antigravity-usage.json"))
	if err != nil {
		t.Fatalf("parseAntigravityUsage() error = %v", err)
	}
	if len(usage.Groups) != 1 || usage.Groups[0].Name != "Gemini" {
		t.Fatalf("Groups = %#v, want one Gemini group", usage.Groups)
	}
	if len(usage.Groups[0].Buckets) != 1 {
		t.Fatalf("Buckets = %#v, want one bucket", usage.Groups[0].Buckets)
	}
	bucket := usage.Groups[0].Buckets[0]
	if bucket.Name != "daily" || bucket.Window != "24h" {
		t.Errorf("bucket identity = (%q, %q), want (daily, 24h)", bucket.Name, bucket.Window)
	}
	if bucket.Remaining != 87.5 {
		t.Errorf("Remaining = %v, want 87.5", bucket.Remaining)
	}
	if bucket.ResetTime != "2026-08-30T00:00:00Z" {
		t.Errorf("ResetTime = %q, want %q", bucket.ResetTime, "2026-08-30T00:00:00Z")
	}
}

func TestParseUsageRejectsInvalidJSON(t *testing.T) {
	if _, err := parseCodexUsage([]byte(`{`)); err == nil {
		t.Error("parseCodexUsage() error = nil, want invalid JSON error")
	}
	if _, err := parseAntigravityUsage([]byte(`{`)); err == nil {
		t.Error("parseAntigravityUsage() error = nil, want invalid JSON error")
	}
}

func TestParseCodexUsageRejectsMissingAndOutOfRangeFields(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":-1,"reset_after_seconds":1},"secondary_window":{"used_percent":1,"reset_after_seconds":1}}}`),
	} {
		if _, err := parseCodexUsage(data); err == nil {
			t.Errorf("parseCodexUsage(%s) error = nil, want validation error", data)
		}
	}
}

func TestParseCodexUsageAllowsMissingPlan(t *testing.T) {
	data := []byte(`{"rate_limit":{"primary_window":{"used_percent":1,"reset_after_seconds":1},"secondary_window":{"used_percent":2,"reset_after_seconds":2}}}`)
	usage, err := parseCodexUsage(data)
	if err != nil {
		t.Fatalf("parseCodexUsage() error = %v", err)
	}
	if usage.Plan != "" {
		t.Errorf("Plan = %q, want empty", usage.Plan)
	}
}
