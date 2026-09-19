package providers

import (
	"context"
	"testing"

	"github.com/jmnote/aigauge/internal/auth"
)

func TestParseCopilotUsage(t *testing.T) {
	data := readFixture(t, "copilot-usage.json")
	usage, err := ParseCopilotUsage(data)
	if err != nil {
		t.Fatalf("ParseCopilotUsage() error = %v", err)
	}
	if usage.CopilotPlan != "individual" {
		t.Errorf("CopilotPlan = %q, want %q", usage.CopilotPlan, "individual")
	}
	if usage.QuotaResetDate != "2026-10-01T00:00:00Z" {
		t.Errorf("QuotaResetDate = %q, want %q", usage.QuotaResetDate, "2026-10-01T00:00:00Z")
	}
	prem, ok := usage.QuotaSnapshots["premium_interactions"]
	if !ok {
		t.Fatalf("missing premium_interactions snapshot")
	}
	if prem.PercentRemaining != 75.0 || prem.Remaining != 375 {
		t.Errorf("premium_interactions = %+v, want 75.0%% and 375 remaining", prem)
	}
}

func TestCopilotToDisplay(t *testing.T) {
	data := readFixture(t, "copilot-usage.json")
	usage, err := ParseCopilotUsage(data)
	if err != nil {
		t.Fatalf("ParseCopilotUsage() error = %v", err)
	}
	usage.Status = StatusConnected
	usage.FetchedAt = "2026-09-19T12:00:00Z"

	display := usage.ToDisplay()
	if display.Status != StatusConnected {
		t.Fatalf("display.Status = %q, want %q", display.Status, StatusConnected)
	}
	if display.Plan != "individual" {
		t.Errorf("display.Plan = %q, want %q", display.Plan, "individual")
	}
	if len(display.Groups) != 1 {
		t.Fatalf("display.Groups count = %d, want 1", len(display.Groups))
	}
	buckets := display.Groups[0].Buckets
	// First bucket should be premium requests with detail formatted with remaining / entitlement
	if buckets[0].Label != "premium requests" || buckets[0].Detail != "375/500" || buckets[0].Remaining != 75.0 {
		t.Errorf("bucket[0] = %+v, want premium requests with 375/500 and 75%%", buckets[0])
	}
	if buckets[0].ResetTime != "2026-10-01T00:00:00Z" {
		t.Errorf("bucket[0].ResetTime = %q, want %q", buckets[0].ResetTime, "2026-10-01T00:00:00Z")
	}

	// Chat and Completions should be omitted, leaving only Copilot
	if len(buckets) != 1 {
		t.Fatalf("expected only 1 bucket, got %d", len(buckets))
	}
}

func TestCopilotToDisplayFallsBackToUnlimitedWithoutPremiumSnapshot(t *testing.T) {
	data := readFixture(t, "copilot-usage-no-premium.json")
	usage, err := ParseCopilotUsage(data)
	if err != nil {
		t.Fatalf("ParseCopilotUsage() error = %v", err)
	}
	usage.Status = StatusConnected
	usage.FetchedAt = "2026-09-19T12:00:00Z"

	display := usage.ToDisplay()
	if display.Status != StatusConnected {
		t.Fatalf("display.Status = %q, want %q", display.Status, StatusConnected)
	}
	if len(display.Groups) != 1 || len(display.Groups[0].Buckets) != 1 {
		t.Fatalf("display.Groups = %+v, want 1 group with 1 bucket", display.Groups)
	}
	bucket := display.Groups[0].Buckets[0]
	if bucket.Detail != "Unlimited" || bucket.Remaining != 100 {
		t.Errorf("bucket = %+v, want Unlimited at 100%%", bucket)
	}
}

func TestCopilotToDisplayDistinguishesPremiumAndCreditsLabels(t *testing.T) {
	data := readFixture(t, "copilot-usage-premium-and-credits.json")
	usage, err := ParseCopilotUsage(data)
	if err != nil {
		t.Fatalf("ParseCopilotUsage() error = %v", err)
	}
	usage.Status = StatusConnected
	usage.FetchedAt = "2026-09-19T12:00:00Z"

	display := usage.ToDisplay()
	buckets := display.Groups[0].Buckets
	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d: %+v", len(buckets), buckets)
	}
	if buckets[0].Label == buckets[1].Label {
		t.Errorf("expected distinct labels, both bucket labels are %q", buckets[0].Label)
	}
	if buckets[0].Label != "premium requests" || buckets[1].Label != "ai credits" {
		t.Errorf("buckets = %+v, want labels %q and %q", buckets, "premium requests", "ai credits")
	}
}

func TestDiagnoseCopilot(t *testing.T) {
	ctx := context.Background()

	// 1. No token -> login required
	depsNoToken := providerDeps{
		getToken: func(string) (*auth.Token, error) {
			return nil, nil
		},
	}
	diag, _, ok := diagnoseCopilot(ctx, depsNoToken, "test-instance", false)
	if ok || diag.Status != StatusLoginRequired {
		t.Errorf("diag without token = %+v, want StatusLoginRequired and not ok", diag)
	}

	// 2. Token present, inactive -> auth check required
	depsWithToken := providerDeps{
		getToken: func(string) (*auth.Token, error) {
			return &auth.Token{AccessToken: "gho_sample_token"}, nil
		},
	}
	diag, creds, ok := diagnoseCopilot(ctx, depsWithToken, "test-instance", false)
	if ok || diag.Status != StatusAuthCheckRequired {
		t.Errorf("diag with token inactive = %+v, want StatusAuthCheckRequired and not ok", diag)
	}
	if creds.Tokens.AccessToken != "gho_sample_token" {
		t.Errorf("creds token = %q, want %q", creds.Tokens.AccessToken, "gho_sample_token")
	}

	// 3. Token present, active -> connected ok
	diagActive, _, okActive := diagnoseCopilot(ctx, depsWithToken, "test-instance", true)
	if !okActive || diagActive.Status != "" {
		t.Errorf("diag with token active = %+v (ok=%v), want ok=true", diagActive, okActive)
	}
}
