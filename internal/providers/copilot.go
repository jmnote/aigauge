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

	// Copilot's usage rate is always a single monthly bucket (Chat/Completions
	// are excluded since they're typically unlimited). The label reflects the
	// reset cadence (like Claude/Codex's "5h"/"7d"), not the source field,
	// since quota_reset_date is monthly either way. No premium_interactions
	// snapshot means no premium request allowance at all, not an unlimited
	// one, so the zero-value bucket (0% remaining, no detail) stands.
	bucket := DisplayUsageBucket{Label: "monthly"}
	if snapshot, ok := u.QuotaSnapshots["premium_interactions"]; ok {
		bucket.Remaining = snapshot.PercentRemaining
		bucket.ResetTime = u.QuotaResetDate
		if snapshot.Entitlement > 0 {
			bucket.Detail = fmt.Sprintf("%d/%d", int(snapshot.Remaining), int(snapshot.Entitlement))
		}
	}

	display.Groups = []DisplayUsageGroup{{Buckets: []DisplayUsageBucket{bucket}}}
	display.Status = StatusConnected
	return display
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
