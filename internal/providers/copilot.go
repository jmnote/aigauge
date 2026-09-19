package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ParseCopilotUsage unmarshals a raw GitHub Copilot internal user response.
func ParseCopilotUsage(data []byte) (CopilotUsage, error) {
	var usage CopilotUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return CopilotUsage{}, err
	}
	usage.Raw = append(json.RawMessage(nil), data...)
	return usage, nil
}

// ToDisplay converts Copilot's quota snapshots and GitHub billing metrics into display buckets.
func (u CopilotUsage) ToDisplay() DisplayUsage {
	display := DisplayUsage{FetchedAt: u.FetchedAt, DiagnosisFields: u.DiagnosisFields}
	if u.Status != StatusConnected {
		return display
	}

	display.Plan = u.CopilotPlan

	if u.CopilotPlan == "" && len(u.QuotaSnapshots) == 0 {
		display.applyDiagnosis(usageUnreadableDiagnosis("GitHub Copilot", ReasonNoUsageData, fmt.Errorf("no quota snapshots available in response")))
		return display
	}

	// Copilot's usage rate prefers ai_credits.percent_remaining, falling back
	// to premium_interactions.percent_remaining, and 0% if neither snapshot is
	// present. A snapshot that has fully run out (remaining == 0) is treated
	// as 0% remaining regardless of what percent_remaining itself reports.
	// The label reflects the reset cadence (like Claude/Codex's "5h"/"7d"),
	// not which snapshot backed the number, since quota_reset_date is monthly
	// either way.
	detail, remaining := "", 0.0
	if credits, ok := u.QuotaSnapshots["ai_credits"]; ok {
		detail = quotaDetail(credits)
		if credits.Remaining != 0 {
			remaining = credits.PercentRemaining
		}
	} else if premium, ok := u.QuotaSnapshots["premium_interactions"]; ok {
		detail = quotaDetail(premium)
		if premium.Remaining != 0 {
			remaining = premium.PercentRemaining
		}
	}

	display.Groups = []DisplayUsageGroup{{
		Buckets: []DisplayUsageBucket{{
			Label:     "monthly",
			Detail:    detail,
			Remaining: remaining,
			ResetTime: u.QuotaResetDate,
		}},
	}}
	display.Status = StatusConnected
	return display
}

func quotaDetail(snapshot CopilotQuotaSnapshot) string {
	if snapshot.Entitlement > 0 {
		return fmt.Sprintf("%d/%d", int(snapshot.Remaining), int(snapshot.Entitlement))
	}
	if snapshot.Remaining > 0 {
		return fmt.Sprintf("%d", int(snapshot.Remaining))
	}
	return ""
}

// GetCopilotUsage fetches the usage for the GitHub Copilot instance identified by tokenKey.
func GetCopilotUsage(tokenKey string) CopilotUsage {
	return getCopilotUsage(context.Background(), defaultDeps(), tokenKey, true)
}

// FetchCopilotRawUsage returns the unconverted usage response for the Copilot
// instance whose token is stored under tokenKey.
func FetchCopilotRawUsage(tokenKey string) ([]byte, error) {
	ctx := context.Background()
	deps := defaultDeps()
	diagnosis, credentials, ok := diagnoseCopilot(ctx, deps, tokenKey, true)
	if !ok {
		return nil, fmt.Errorf("%s", diagnosis.Message)
	}
	accessToken, err := authorizedAccessToken(ctx, deps, "copilot", tokenKey, credentials.Tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	return fetchAuthorizedJSON(ctx, "https://api.github.com/copilot_internal/user", "GitHub Copilot", map[string]string{
		"Authorization": "Bearer " + accessToken,
		"Accept":        "application/json",
		"User-Agent":    "AI-Gauge",
	})
}

func getCopilotUsage(ctx context.Context, deps providerDeps, tokenKey string, active bool) CopilotUsage {
	usage := CopilotUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	diagnosis, credentials, ok := diagnoseCopilot(ctx, deps, tokenKey, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	accessToken, err := authorizedAccessToken(ctx, deps, "copilot", tokenKey, credentials.Tokens.AccessToken)
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("GitHub Copilot", err))
		return usage
	}

	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
		"Accept":        "application/json",
		"User-Agent":    "AI-Gauge",
	}

	data, err := fetchAuthorizedJSON(ctx, "https://api.github.com/copilot_internal/user", "GitHub Copilot", headers)
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("GitHub Copilot", err))
		return usage
	}

	parsed, err := ParseCopilotUsage(data)
	if err != nil {
		usage.applyDiagnosis(usageUnreadableDiagnosis("GitHub Copilot", ReasonUnsupportedResponse, err))
		return usage
	}

	parsed.FetchedAt = usage.FetchedAt
	parsed.Status = StatusConnected
	return parsed
}
