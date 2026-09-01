package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ClaudeUsage struct {
	Plan      string              `json:"plan"`
	Buckets   []ClaudeUsageBucket `json:"buckets"`
	FetchedAt string              `json:"fetchedAt"`
	Error     string              `json:"error,omitempty"`
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
		return ClaudeUsage{}, fmt.Errorf("response contains no usable usage windows")
	}
	return usage, nil
}

func GetClaudeUsage() ClaudeUsage {
	usage := ClaudeUsage{FetchedAt: time.Now().Format(time.RFC3339)}
	home, err := os.UserHomeDir()
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to get home directory: %v", err)
		return usage
	}

	credPath := filepath.Join(home, ".claude", ".credentials.json")
	credData, err := os.ReadFile(credPath)
	if err != nil {
		usage.Error = "Claude Code login information not found (~/.claude/.credentials.json)"
		return usage
	}
	var creds claudeCredentials
	if err := json.Unmarshal(credData, &creds); err != nil || creds.ClaudeAiOauth.AccessToken == "" {
		usage.Error = "Unable to read Claude Code access token"
		return usage
	}

	body, err := fetchAuthorizedJSON("https://api.anthropic.com/api/oauth/usage", "Claude", map[string]string{
		"Authorization":  "Bearer " + creds.ClaudeAiOauth.AccessToken,
		"anthropic-beta": "oauth-2025-04-20",
	})
	if err != nil {
		usage.Error = err.Error()
		return usage
	}

	parsed, err := parseClaudeUsage(body)
	if err != nil {
		usage.Error = fmt.Sprintf("Unable to parse Claude usage response: %v", err)
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	parsed.Plan = creds.ClaudeAiOauth.SubscriptionType
	return parsed
}
