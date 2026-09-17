package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var errNoUsageWindows = errors.New("response contains no usable usage windows")

// ParseClaudeUsage unmarshals a raw Claude usage response with no
// validation or conversion - see ToDisplay for that.
func ParseClaudeUsage(data []byte) (ClaudeUsage, error) {
	var usage ClaudeUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return ClaudeUsage{}, err
	}
	usage.Raw = append(json.RawMessage(nil), data...)
	return usage, nil
}

func displayBucket(label string, window claudeUsageWindow) (DisplayUsageBucket, error) {
	if window.Utilization == nil {
		return DisplayUsageBucket{}, fmt.Errorf("missing utilization for %s window", label)
	}
	utilization := *window.Utilization
	if utilization < 0 || utilization > 100 {
		return DisplayUsageBucket{}, fmt.Errorf("utilization out of range for %s window", label)
	}
	return DisplayUsageBucket{
		Label:     label,
		Remaining: 100 - utilization,
		ResetTime: window.ResetsAt,
	}, nil
}

// ToDisplay validates each window (5h/7d required, the per-model weekly
// windows optional and undocumented) and converts utilization into
// remaining percentage.
func (u ClaudeUsage) ToDisplay() DisplayUsage {
	display := DisplayUsage{Plan: u.Plan, FetchedAt: u.FetchedAt, DiagnosisFields: u.DiagnosisFields}
	if u.Status != StatusConnected {
		return display
	}

	var buckets []DisplayUsageBucket
	// The 5h/7d windows are normally always present, but this is an
	// undocumented endpoint, so treat an absent window as "not reported for
	// this account" rather than an error: skip it and keep whichever
	// windows did come back, only failing when a present window fails
	// validation (e.g. an out-of-range utilization), which does indicate a
	// real parsing problem.
	for _, w := range []struct {
		label  string
		window claudeUsageWindow
	}{
		{"5h", u.FiveHour},
		{"7d", u.SevenDay},
	} {
		if w.window.Utilization == nil {
			continue
		}
		bucket, err := displayBucket(w.label, w.window)
		if err != nil {
			display.applyDiagnosis(usageUnreadableDiagnosis("Claude", ReasonUnsupportedResponse, err))
			return display
		}
		buckets = append(buckets, bucket)
	}

	// The per-model weekly windows are optional and undocumented; skip a
	// malformed one instead of failing the whole response.
	if u.SevenDayOpus != nil {
		if bucket, err := displayBucket("7d (Opus)", *u.SevenDayOpus); err == nil {
			buckets = append(buckets, bucket)
		}
	}
	if u.SevenDaySonnet != nil {
		if bucket, err := displayBucket("7d (Sonnet)", *u.SevenDaySonnet); err == nil {
			buckets = append(buckets, bucket)
		}
	}

	if len(buckets) == 0 {
		display.applyDiagnosis(usageUnreadableDiagnosis("Claude", ReasonNoUsageData, errNoUsageWindows))
		return display
	}
	display.Groups = []DisplayUsageGroup{{Buckets: buckets}}
	display.Status = StatusConnected
	return display
}

func GetClaudeUsage(tokenKey string) ClaudeUsage {
	return getClaudeUsage(context.Background(), defaultDeps(), tokenKey, true)
}

// FetchClaudeRawUsage returns the unconverted usage response for the Claude
// instance whose token is stored under tokenKey, using the same auth/HTTP
// path as GetClaudeUsage. Used by hack/fixtures/fixtures.go to capture the
// API's actual response shape for fixture development.
func FetchClaudeRawUsage(tokenKey string) ([]byte, error) {
	ctx := context.Background()
	deps := defaultDeps()
	diagnosis, credentials, ok := diagnoseClaude(ctx, deps, tokenKey, true)
	if !ok {
		return nil, fmt.Errorf("%s", diagnosis.Message)
	}
	accessToken, err := authorizedAccessToken(ctx, deps, "claude", tokenKey, credentials.ClaudeAiOauth.AccessToken)
	if err != nil {
		return nil, err
	}
	return fetchAuthorizedJSON(ctx, "https://api.anthropic.com/api/oauth/usage", "Claude", map[string]string{
		"Authorization":  "Bearer " + accessToken,
		"anthropic-beta": "oauth-2025-04-20",
	})
}

func getClaudeUsage(ctx context.Context, deps providerDeps, tokenKey string, active bool) ClaudeUsage {
	usage := ClaudeUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	diagnosis, credentials, ok := diagnoseClaude(ctx, deps, tokenKey, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	accessToken, err := authorizedAccessToken(ctx, deps, "claude", tokenKey, credentials.ClaudeAiOauth.AccessToken)
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Claude", err))
		return usage
	}
	body, err := fetchAuthorizedJSON(ctx, "https://api.anthropic.com/api/oauth/usage", "Claude", map[string]string{
		"Authorization":  "Bearer " + accessToken,
		"anthropic-beta": "oauth-2025-04-20",
	})
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Claude", err))
		return usage
	}

	parsed, err := ParseClaudeUsage(body)
	if err != nil {
		usage.applyDiagnosis(usageUnreadableDiagnosis("Claude", ReasonUnsupportedResponse, err))
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	parsed.Plan = credentials.ClaudeAiOauth.SubscriptionType
	parsed.Status = StatusConnected
	return parsed
}
