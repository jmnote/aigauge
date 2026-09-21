package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParseCodexUsage unmarshals a raw Codex usage response with no validation
// or conversion - see ToDisplay for that.
func ParseCodexUsage(data []byte) (CodexUsage, error) {
	var usage CodexUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return CodexUsage{}, err
	}
	usage.Raw = append(json.RawMessage(nil), data...)
	return usage, nil
}

// ToDisplay validates the raw rate_limit fields (present, 0-100 for a
// percentage, non-negative for a reset) and converts Codex's relative-
// seconds resets into absolute timestamps anchored to FetchedAt.
func (u CodexUsage) ToDisplay() DisplayUsage {
	email := strings.TrimSpace(u.Email)
	display := DisplayUsage{FetchedAt: u.FetchedAt, User: email, Email: email, ResetCredits: u.RateLimitResetCredits.AvailableCount, DiagnosisFields: u.DiagnosisFields}
	if u.Status != StatusConnected {
		return display
	}

	primary := u.RateLimit.PrimaryWindow
	secondary := u.RateLimit.SecondaryWindow
	for _, window := range []struct {
		name  string
		used  *float64
		reset *int
	}{
		{"primary_window", primary.UsedPercent, primary.ResetAfterSeconds},
		{"secondary_window", secondary.UsedPercent, secondary.ResetAfterSeconds},
	} {
		var err error
		switch {
		case window.used == nil:
			err = fmt.Errorf("rate_limit.%s.used_percent is missing or null", window.name)
		case window.reset == nil:
			err = fmt.Errorf("rate_limit.%s.reset_after_seconds is missing or null", window.name)
		case *window.used < 0 || *window.used > 100:
			err = fmt.Errorf("rate_limit.%s.used_percent = %g; expected 0-100", window.name, *window.used)
		case *window.reset < 0:
			err = fmt.Errorf("rate_limit.%s.reset_after_seconds = %d; expected >= 0", window.name, *window.reset)
		}
		if err != nil {
			display.applyDiagnosis(usageUnreadableDiagnosis("Codex", ReasonUnsupportedResponse, err))
			return display
		}
	}

	now := time.Now()
	if parsed, err := time.Parse(time.RFC3339, u.FetchedAt); err == nil {
		now = parsed
	}
	toResetTime := func(seconds int) string {
		if seconds <= 0 {
			return ""
		}
		return now.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
	}

	display.Plan = u.PlanType
	display.Groups = []DisplayUsageGroup{{
		Buckets: []DisplayUsageBucket{
			{Label: "5h", Remaining: 100 - *primary.UsedPercent, ResetTime: toResetTime(*primary.ResetAfterSeconds)},
			{Label: "7d", Remaining: 100 - *secondary.UsedPercent, ResetTime: toResetTime(*secondary.ResetAfterSeconds)},
		},
	}}
	display.Status = StatusConnected
	return display
}

func GetCodexUsage(tokenKey string) CodexUsage {
	return getCodexUsage(context.Background(), defaultDeps(), tokenKey, true)
}

// FetchCodexRawUsage returns the unconverted usage response for the Codex
// instance whose token is stored under tokenKey, using the same auth/HTTP
// path as GetCodexUsage. Used by hack/fixtures/fixtures.go to capture the
// API's actual response shape for fixture development.
func FetchCodexRawUsage(tokenKey string) ([]byte, error) {
	ctx := context.Background()
	deps := defaultDeps()
	diagnosis, credentials, ok := diagnoseCodex(ctx, deps, tokenKey, true)
	if !ok {
		return nil, fmt.Errorf("%s", diagnosis.Message)
	}
	accessToken, err := authorizedAccessToken(ctx, deps, "codex", tokenKey, credentials.Tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	return fetchAuthorizedJSON(ctx, "https://chatgpt.com/backend-api/wham/usage", "Codex", map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
}

func getCodexUsage(ctx context.Context, deps providerDeps, tokenKey string, active bool) CodexUsage {
	usage := CodexUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	diagnosis, credentials, ok := diagnoseCodex(ctx, deps, tokenKey, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	accessToken, err := authorizedAccessToken(ctx, deps, "codex", tokenKey, credentials.Tokens.AccessToken)
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Codex", err))
		return usage
	}
	body, err := fetchAuthorizedJSON(ctx, "https://chatgpt.com/backend-api/wham/usage", "Codex", map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Codex", err))
		return usage
	}

	parsed, err := ParseCodexUsage(body)
	if err != nil {
		usage.applyDiagnosis(usageUnreadableDiagnosis("Codex", ReasonUnsupportedResponse, err))
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	parsed.Status = StatusConnected
	return parsed
}
