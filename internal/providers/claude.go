package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// errNoUsageWindows marks a usage response that parsed cleanly but described no
// window to show. It is a sentinel rather than a plain error string so the
// caller can tell "this account has nothing to report" apart from "this
// response is a shape we cannot read" when choosing the Reason to surface.
var errNoUsageWindows = errors.New("response contains no usable usage windows")

type ClaudeUsage struct {
	Plan      string              `json:"plan"`
	Buckets   []ClaudeUsageBucket `json:"buckets"`
	FetchedAt string              `json:"fetchedAt"`
	Error     string              `json:"error,omitempty"`

	// Status and friends carry the structured diagnosis. Error is still filled
	// in alongside them so the current frontend, which only knows how to read a
	// message string, keeps working until it switches to Status.
	Status  Status `json:"status,omitempty"`
	Reason  Reason `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"details,omitempty"`
}

func (u *ClaudeUsage) applyDiagnosis(diagnosis Diagnosis) {
	u.Status = diagnosis.Status
	u.Reason = diagnosis.Reason
	u.Message = diagnosis.Message
	u.Details = diagnosis.Details
	u.Error = diagnosis.Message
}

type ClaudeUsageBucket struct {
	Name      string  `json:"name"`
	Remaining float64 `json:"remaining"`
	ResetTime string  `json:"resetTime"`
}

type claudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}

type claudeUsageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

type claudeUsageResponse struct {
	FiveHour       claudeUsageWindow  `json:"five_hour"`
	SevenDay       claudeUsageWindow  `json:"seven_day"`
	SevenDayOpus   *claudeUsageWindow `json:"seven_day_opus"`
	SevenDaySonnet *claudeUsageWindow `json:"seven_day_sonnet"`
}

func newClaudeBucket(name string, window claudeUsageWindow) (ClaudeUsageBucket, error) {
	if window.Utilization == nil {
		return ClaudeUsageBucket{}, fmt.Errorf("missing utilization for %s window", name)
	}
	utilization := *window.Utilization
	if utilization < 0 || utilization > 100 {
		return ClaudeUsageBucket{}, fmt.Errorf("utilization out of range for %s window", name)
	}
	return ClaudeUsageBucket{
		Name:      name,
		Remaining: 100 - utilization,
		ResetTime: window.ResetsAt,
	}, nil
}

func parseClaudeUsage(data []byte) (ClaudeUsage, error) {
	var response claudeUsageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return ClaudeUsage{}, err
	}

	usage := ClaudeUsage{}

	// The 5h/7d windows are normally always present, but this is an undocumented
	// endpoint, so treat an absent window as "not reported for this account" rather
	// than an error: skip it and keep whichever windows did come back, only
	// propagating an error when the window is present but fails validation (e.g.
	// an out-of-range utilization), which does indicate a real parsing problem.
	for _, w := range []struct {
		name   string
		window claudeUsageWindow
	}{
		{"5h", response.FiveHour},
		{"7d", response.SevenDay},
	} {
		if w.window.Utilization == nil {
			continue
		}
		bucket, err := newClaudeBucket(w.name, w.window)
		if err != nil {
			return ClaudeUsage{}, err
		}
		usage.Buckets = append(usage.Buckets, bucket)
	}

	// The per-model weekly windows are optional and undocumented; skip a malformed
	// one instead of failing the whole response.
	if response.SevenDayOpus != nil {
		if bucket, err := newClaudeBucket("7d (Opus)", *response.SevenDayOpus); err == nil {
			usage.Buckets = append(usage.Buckets, bucket)
		}
	}
	if response.SevenDaySonnet != nil {
		if bucket, err := newClaudeBucket("7d (Sonnet)", *response.SevenDaySonnet); err == nil {
			usage.Buckets = append(usage.Buckets, bucket)
		}
	}

	if len(usage.Buckets) == 0 {
		return ClaudeUsage{}, errNoUsageWindows
	}
	return usage, nil
}

// findClaudeCredentials reads the minimum this app needs to call the usage API:
// the access token, plus the subscription label shown on the card. It reports
// found=false both when the file is absent and when it cannot be parsed - the
// caller treats those the same way, by falling back to the CLI for an
// explanation - and it never returns the file's other contents.
func findClaudeCredentials(homeDir func() (string, error), readFile func(string) ([]byte, error)) (claudeCredentials, bool) {
	home, err := homeDir()
	if err != nil {
		return claudeCredentials{}, false
	}
	data, err := readFile(filepath.Join(home, ".claude", ".credentials.json"))
	if err != nil {
		return claudeCredentials{}, false
	}
	var credentials claudeCredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return claudeCredentials{}, false
	}
	if credentials.ClaudeAiOauth.AccessToken == "" {
		return claudeCredentials{}, false
	}
	return credentials, true
}

func GetClaudeUsage() ClaudeUsage {
	return getClaudeUsage(context.Background(), defaultDeps(), true)
}

func getClaudeUsage(ctx context.Context, deps providerDeps, active bool) ClaudeUsage {
	usage := ClaudeUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	diagnosis, credentials, ok := diagnoseClaude(ctx, deps, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	body, err := fetchAuthorizedJSON("https://api.anthropic.com/api/oauth/usage", "Claude", map[string]string{
		"Authorization":  "Bearer " + credentials.ClaudeAiOauth.AccessToken,
		"anthropic-beta": "oauth-2025-04-20",
	})
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Claude", err))
		return usage
	}

	parsed, err := parseClaudeUsage(body)
	if err != nil {
		// An account that simply reports no window is a different problem from a
		// response shape this version cannot read: the first is nothing the user
		// can act on, the second may be fixed by an update.
		reason := ReasonUnsupportedResponse
		if errors.Is(err, errNoUsageWindows) {
			reason = ReasonNoUsageData
		}
		usage.applyDiagnosis(usageUnreadableDiagnosis("Claude", reason, err))
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	parsed.Plan = credentials.ClaudeAiOauth.SubscriptionType
	parsed.Status = StatusConnected
	return parsed
}
