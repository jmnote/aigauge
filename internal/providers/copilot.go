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

	var buckets []DisplayUsageBucket

	// 1. Copilot Premium Requests / AI Credits
	// Chat and Completions are intentionally excluded as they are typically unlimited.
	copilotKeys := []string{"premium_interactions", "ai_credits"}

	for _, key := range copilotKeys {
		if snapshot, ok := u.QuotaSnapshots[key]; ok {
			remPercent := snapshot.PercentRemaining
			if snapshot.Unlimited {
				remPercent = 100
			}
			label := "monthly"
			detail := ""
			if snapshot.Entitlement > 0 {
				detail = fmt.Sprintf("%d/%d", int(snapshot.Remaining), int(snapshot.Entitlement))
			} else if snapshot.Remaining > 0 {
				detail = fmt.Sprintf("%d", int(snapshot.Remaining))
			}
			buckets = append(buckets, DisplayUsageBucket{
				Label:     label,
				Detail:    detail,
				Remaining: remPercent,
				ResetTime: u.QuotaResetDate,
			})
			break
		}
	}

	if len(buckets) == 0 {
		display.applyDiagnosis(usageUnreadableDiagnosis("GitHub Copilot", ReasonNoUsageData, fmt.Errorf("no quota snapshots available in response")))
		return display
	}

	display.Groups = []DisplayUsageGroup{{
		Buckets: buckets,
	}}
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
