package providers

import (
	"errors"
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

func TestResolveAgyPathReturnsLookupPathWhenFound(t *testing.T) {
	calledFallback := false
	path, err := resolveAgyPath("/usr/bin/agy", nil, func() (string, error) {
		calledFallback = true
		return "", nil
	}, func(string) bool {
		calledFallback = true
		return false
	})
	if err != nil || path != "/usr/bin/agy" {
		t.Errorf("resolveAgyPath() = (%q, %v), want (\"/usr/bin/agy\", nil)", path, err)
	}
	if calledFallback {
		t.Error("resolveAgyPath() consulted the fallback path when the lookup already succeeded")
	}
}

func TestResolveAgyPathErrorsWhenHomeDirFails(t *testing.T) {
	_, err := resolveAgyPath("", errors.New("not found"), func() (string, error) {
		return "", errors.New("no home dir")
	}, func(string) bool { return true })
	if err == nil {
		t.Fatal("resolveAgyPath() error = nil, want an error when the home directory can't be determined")
	}
}

func TestResolveAgyPathFallsBackWhenLookupFails(t *testing.T) {
	home := filepath.Join("C:", "Users", "test")
	wantPath, supported := antigravityFallbackPath(home)

	path, err := resolveAgyPath("", errors.New("not found"), func() (string, error) {
		return home, nil
	}, func(string) bool { return true })

	if !supported {
		if err == nil {
			t.Fatal("resolveAgyPath() error = nil, want an error on platforms without a fallback path")
		}
		return
	}
	if err != nil || path != wantPath {
		t.Errorf("resolveAgyPath() = (%q, %v), want (%q, nil)", path, err, wantPath)
	}
}

func TestResolveAgyPathErrorsWhenFallbackDoesNotExist(t *testing.T) {
	home := filepath.Join("C:", "Users", "test")
	_, supported := antigravityFallbackPath(home)
	if !supported {
		t.Skip("no fallback path is defined for this platform")
	}

	_, err := resolveAgyPath("", errors.New("not found"), func() (string, error) {
		return home, nil
	}, func(string) bool { return false })
	if err == nil {
		t.Fatal("resolveAgyPath() error = nil, want an error when the fallback binary doesn't exist on disk")
	}
}

func TestParseClaudeUsage(t *testing.T) {
	usage, err := parseClaudeUsage(readFixture(t, "claude-usage.json"))
	if err != nil {
		t.Fatalf("parseClaudeUsage() error = %v", err)
	}
	if len(usage.Buckets) != 3 {
		t.Fatalf("Buckets = %#v, want 3 buckets", usage.Buckets)
	}
	fiveHour, sevenDay, sevenDayOpus := usage.Buckets[0], usage.Buckets[1], usage.Buckets[2]
	if fiveHour.Name != "5h" || fiveHour.Remaining != 62.5 {
		t.Errorf("five hour bucket = %#v, want (5h, 62.5)", fiveHour)
	}
	if sevenDay.Name != "7d" || sevenDay.Remaining != 37.75 {
		t.Errorf("seven day bucket = %#v, want (7d, 37.75)", sevenDay)
	}
	if sevenDayOpus.Name != "7d (Opus)" || sevenDayOpus.Remaining != 90 {
		t.Errorf("seven day opus bucket = %#v, want (7d (Opus), 90)", sevenDayOpus)
	}
	if fiveHour.ResetTime != "2026-08-30T05:00:00Z" {
		t.Errorf("ResetTime = %q, want %q", fiveHour.ResetTime, "2026-08-30T05:00:00Z")
	}
}

func TestParseClaudeUsageDegradesWhenAWindowIsAbsent(t *testing.T) {
	data := []byte(`{"five_hour":{"utilization":10,"resets_at":"x"}}`)
	usage, err := parseClaudeUsage(data)
	if err != nil {
		t.Fatalf("parseClaudeUsage() error = %v, want the present 5h window to still parse", err)
	}
	if len(usage.Buckets) != 1 || usage.Buckets[0].Name != "5h" {
		t.Fatalf("Buckets = %#v, want just the 5h window", usage.Buckets)
	}
}

func TestParseClaudeUsageSkipsMalformedOptionalWindow(t *testing.T) {
	data := []byte(`{"five_hour":{"utilization":10,"resets_at":"x"},"seven_day":{"utilization":20,"resets_at":"y"},"seven_day_opus":{"utilization":150,"resets_at":"z"}}`)
	usage, err := parseClaudeUsage(data)
	if err != nil {
		t.Fatalf("parseClaudeUsage() error = %v, want required 5h/7d buckets to still parse", err)
	}
	if len(usage.Buckets) != 2 {
		t.Fatalf("Buckets = %#v, want the malformed optional 7d (Opus) window skipped", usage.Buckets)
	}
}

func TestParseClaudeUsageRejectsMissingAndOutOfRangeFields(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"five_hour":{"utilization":-1,"resets_at":"x"},"seven_day":{"utilization":1,"resets_at":"x"}}`),
	} {
		if _, err := parseClaudeUsage(data); err == nil {
			t.Errorf("parseClaudeUsage(%s) error = nil, want validation error", data)
		}
	}
}

func TestParseUsageRejectsInvalidJSON(t *testing.T) {
	if _, err := parseCodexUsage([]byte(`{`)); err == nil {
		t.Error("parseCodexUsage() error = nil, want invalid JSON error")
	}
	if _, err := parseAntigravityUsage([]byte(`{`)); err == nil {
		t.Error("parseAntigravityUsage() error = nil, want invalid JSON error")
	}
	if _, err := parseClaudeUsage([]byte(`{`)); err == nil {
		t.Error("parseClaudeUsage() error = nil, want invalid JSON error")
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
