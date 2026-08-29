package providers

import "testing"

func TestParseCodexUsage(t *testing.T) {
	data := []byte(`{
      "plan_type": "pro",
      "rate_limit": {
        "primary_window": {"used_percent": 37.5, "reset_after_seconds": 3600},
        "secondary_window": {"used_percent": 62.25, "reset_after_seconds": 86400}
      }
    }`)

	usage, err := parseCodexUsage(data)
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
	data := []byte(`{
      "command": {
        "data": {
          "groups": [{
            "name": "Gemini",
            "buckets": [{
              "name": "daily",
              "window": "24h",
              "remaining_fraction": 0.875,
              "reset_time": "2026-08-30T00:00:00Z"
            }]
          }]
        }
      }
    }`)

	usage, err := parseAntigravityUsage(data)
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
