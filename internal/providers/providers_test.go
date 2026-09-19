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
	usage, err := ParseCodexUsage(readFixture(t, "codex-usage.json"))
	if err != nil {
		t.Fatalf("ParseCodexUsage() error = %v", err)
	}
	if usage.PlanType != "pro" {
		t.Errorf("PlanType = %q, want %q", usage.PlanType, "pro")
	}
	if usage.Email != "alex@example.com" {
		t.Errorf("Email = %q, want %q", usage.Email, "alex@example.com")
	}
	if usage.RateLimitResetCredits.AvailableCount == nil || *usage.RateLimitResetCredits.AvailableCount != 7 {
		t.Errorf("AvailableCount = %#v, want 7", usage.RateLimitResetCredits.AvailableCount)
	}
	if *usage.RateLimit.PrimaryWindow.UsedPercent != 37.5 || *usage.RateLimit.SecondaryWindow.UsedPercent != 62.25 {
		t.Errorf("used percentages = (%v, %v), want (37.5, 62.25)", *usage.RateLimit.PrimaryWindow.UsedPercent, *usage.RateLimit.SecondaryWindow.UsedPercent)
	}
	if *usage.RateLimit.PrimaryWindow.ResetAfterSeconds != 3600 || *usage.RateLimit.SecondaryWindow.ResetAfterSeconds != 86400 {
		t.Errorf("reset seconds = (%d, %d), want (3600, 86400)", *usage.RateLimit.PrimaryWindow.ResetAfterSeconds, *usage.RateLimit.SecondaryWindow.ResetAfterSeconds)
	}
}

func TestParseAntigravityUsage(t *testing.T) {
	usage, err := ParseAntigravityUsage(readFixture(t, "antigravity-usage.json"))
	if err != nil {
		t.Fatalf("ParseAntigravityUsage() error = %v", err)
	}
	if len(usage.Groups) != 1 || usage.Groups[0].DisplayName != "Gemini" {
		t.Fatalf("Groups = %#v, want one Gemini group", usage.Groups)
	}
	if len(usage.Groups[0].Buckets) != 1 {
		t.Fatalf("Buckets = %#v, want one bucket", usage.Groups[0].Buckets)
	}
	bucket := usage.Groups[0].Buckets[0]
	if bucket.DisplayName != "daily" || bucket.Window != "24h" {
		t.Errorf("bucket identity = (%q, %q), want (daily, 24h)", bucket.DisplayName, bucket.Window)
	}
	if bucket.RemainingFraction != 0.875 {
		t.Errorf("RemainingFraction = %v, want 0.875", bucket.RemainingFraction)
	}
	if bucket.ResetTime != "2026-08-30T00:00:00Z" {
		t.Errorf("ResetTime = %q, want %q", bucket.ResetTime, "2026-08-30T00:00:00Z")
	}
}

func TestParseClaudeUsage(t *testing.T) {
	usage, err := ParseClaudeUsage(readFixture(t, "claude-usage.json"))
	if err != nil {
		t.Fatalf("ParseClaudeUsage() error = %v", err)
	}
	if *usage.FiveHour.Utilization != 37.5 || usage.FiveHour.ResetsAt != "2026-08-30T05:00:00Z" {
		t.Errorf("FiveHour = %#v, want (37.5, 2026-08-30T05:00:00Z)", usage.FiveHour)
	}
	if *usage.SevenDay.Utilization != 62.25 {
		t.Errorf("SevenDay.Utilization = %v, want 62.25", *usage.SevenDay.Utilization)
	}
	if usage.SevenDayOpus == nil || *usage.SevenDayOpus.Utilization != 10 {
		t.Errorf("SevenDayOpus = %#v, want utilization 10", usage.SevenDayOpus)
	}
	if usage.SevenDaySonnet != nil {
		t.Errorf("SevenDaySonnet = %#v, want nil (absent from the fixture)", usage.SevenDaySonnet)
	}
}

func TestParseUsageRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseCodexUsage([]byte(`{`)); err == nil {
		t.Error("ParseCodexUsage() error = nil, want invalid JSON error")
	}
	if _, err := ParseAntigravityUsage([]byte(`{`)); err == nil {
		t.Error("ParseAntigravityUsage() error = nil, want invalid JSON error")
	}
	if _, err := ParseClaudeUsage([]byte(`{`)); err == nil {
		t.Error("ParseClaudeUsage() error = nil, want invalid JSON error")
	}
}

// connectedCodex/connectedClaude parse data and mark the result StatusConnected,
// the state ToDisplay requires to do any conversion - mirroring what
// getCodexUsage/getClaudeUsage set after a successful fetch, since
// ToDisplay itself no longer does that (see ParseXUsage's doc comment).

func connectedCodex(t *testing.T, data []byte) CodexUsage {
	t.Helper()
	usage, err := ParseCodexUsage(data)
	if err != nil {
		t.Fatalf("ParseCodexUsage() error = %v", err)
	}
	usage.Status = StatusConnected
	return usage
}

func connectedClaude(t *testing.T, data []byte) ClaudeUsage {
	t.Helper()
	usage, err := ParseClaudeUsage(data)
	if err != nil {
		t.Fatalf("ParseClaudeUsage() error = %v", err)
	}
	usage.Status = StatusConnected
	return usage
}

func TestCodexToDisplay(t *testing.T) {
	display := connectedCodex(t, readFixture(t, "codex-usage.json")).ToDisplay()
	if display.Error != "" {
		t.Fatalf("ToDisplay() error = %s", display.Error)
	}
	if display.Plan != "pro" {
		t.Errorf("Plan = %q, want %q", display.Plan, "pro")
	}
	if display.ResetCredits == nil || *display.ResetCredits != 7 {
		t.Errorf("ResetCredits = %#v, want 7", display.ResetCredits)
	}
	const expectedEmail = "alex@example.com"
	if display.User != expectedEmail {
		t.Errorf("User = %q, want full email %q", display.User, expectedEmail)
	}
	if display.Email != expectedEmail {
		t.Errorf("Email = %q, want full email %q", display.Email, expectedEmail)
	}
	if len(display.Groups) != 1 || len(display.Groups[0].Buckets) != 2 {
		t.Fatalf("Groups = %#v, want one group with 5h/7d buckets", display.Groups)
	}
	fiveHour, sevenDay := display.Groups[0].Buckets[0], display.Groups[0].Buckets[1]
	if fiveHour.Label != "5h" || fiveHour.Remaining != 62.5 {
		t.Errorf("5h bucket = %#v, want (5h, 62.5)", fiveHour)
	}
	if sevenDay.Label != "7d" || sevenDay.Remaining != 37.75 {
		t.Errorf("7d bucket = %#v, want (7d, 37.75)", sevenDay)
	}
}

func TestClaudeToDisplayUsesAccountDisplayName(t *testing.T) {
	usage := connectedClaude(t, []byte(`{"five_hour":{"utilization":10}}`))
	usage.AccountDisplayName = "Jane"
	if got := usage.ToDisplay().User; got != "Jane" {
		t.Errorf("User = %q, want %q", got, "Jane")
	}
	if got := usage.ToDisplay().DisplayName; got != "Jane" {
		t.Errorf("DisplayName = %q, want %q", got, "Jane")
	}
}

func TestParseClaudeProfileIdentity(t *testing.T) {
	got, err := parseClaudeProfileIdentity([]byte(`{"account":{"display_name":"Jane"}}`))
	if err != nil {
		t.Fatalf("parseClaudeProfileIdentity() error = %v", err)
	}
	if got != "Jane" {
		t.Errorf("display name = %q, want profile display name", got)
	}
}

func TestCodexToDisplayRejectsMissingAndOutOfRangeFields(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":-1,"reset_after_seconds":1},"secondary_window":{"used_percent":1,"reset_after_seconds":1}}}`),
	} {
		display := connectedCodex(t, data).ToDisplay()
		if display.Error == "" {
			t.Errorf("ToDisplay(%s).Error = \"\", want a validation error", data)
		}
	}
}

func TestCodexToDisplayAllowsMissingPlan(t *testing.T) {
	data := []byte(`{"rate_limit":{"primary_window":{"used_percent":1,"reset_after_seconds":1},"secondary_window":{"used_percent":2,"reset_after_seconds":2}}}`)
	display := connectedCodex(t, data).ToDisplay()
	if display.Error != "" {
		t.Fatalf("ToDisplay() error = %s", display.Error)
	}
	if display.Plan != "" {
		t.Errorf("Plan = %q, want empty", display.Plan)
	}
}

func TestClaudeToDisplayDegradesWhenAWindowIsAbsent(t *testing.T) {
	data := []byte(`{"five_hour":{"utilization":10,"resets_at":"x"}}`)
	display := connectedClaude(t, data).ToDisplay()
	if display.Error != "" {
		t.Fatalf("ToDisplay() error = %s, want the present 5h window to still convert", display.Error)
	}
	if len(display.Groups) != 1 || len(display.Groups[0].Buckets) != 1 || display.Groups[0].Buckets[0].Label != "5h" {
		t.Fatalf("Groups = %#v, want just the 5h bucket", display.Groups)
	}
}

func TestClaudeToDisplaySkipsMalformedOptionalWindow(t *testing.T) {
	data := []byte(`{"five_hour":{"utilization":10,"resets_at":"x"},"seven_day":{"utilization":20,"resets_at":"y"},"seven_day_opus":{"utilization":150,"resets_at":"z"}}`)
	display := connectedClaude(t, data).ToDisplay()
	if display.Error != "" {
		t.Fatalf("ToDisplay() error = %s, want required 5h/7d buckets to still convert", display.Error)
	}
	if len(display.Groups) != 1 || len(display.Groups[0].Buckets) != 2 {
		t.Fatalf("Groups = %#v, want the malformed optional 7d (Opus) window skipped", display.Groups)
	}
}

func TestClaudeToDisplayRejectsMissingAndOutOfRangeFields(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"five_hour":{"utilization":-1,"resets_at":"x"},"seven_day":{"utilization":1,"resets_at":"x"}}`),
	} {
		display := connectedClaude(t, data).ToDisplay()
		if display.Error == "" {
			t.Errorf("ToDisplay(%s).Error = \"\", want a validation error", data)
		}
	}
}

func TestClaudeToDisplayNoWindowsReasonIsNoUsageData(t *testing.T) {
	display := connectedClaude(t, []byte(`{}`)).ToDisplay()
	if display.Reason != ReasonNoUsageData {
		t.Errorf("Reason = %q, want %q", display.Reason, ReasonNoUsageData)
	}
	if !errors.Is(errNoUsageWindows, errNoUsageWindows) {
		t.Fatal("sanity: errNoUsageWindows should be comparable to itself")
	}
}
