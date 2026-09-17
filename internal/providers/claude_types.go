package providers

import "encoding/json"

// ClaudeUsage mirrors https://api.anthropic.com/api/oauth/usage's response
// field-for-field - this is what a raw hack/fixtures/usage/usage_claude_*.json
// fixture looks like. Plan and FetchedAt aren't part of that response: Plan
// comes from the credentials file's subscriptionType and FetchedAt is
// stamped on at fetch time, both attached after the fact. ToDisplay
// (claude.go) does all validation and unit conversion; this type does none.
type ClaudeUsage struct {
	FiveHour       claudeUsageWindow  `json:"five_hour"`
	SevenDay       claudeUsageWindow  `json:"seven_day"`
	SevenDayOpus   *claudeUsageWindow `json:"seven_day_opus"`
	SevenDaySonnet *claudeUsageWindow `json:"seven_day_sonnet"`
	Plan           string             `json:"plan"`
	FetchedAt      string             `json:"fetchedAt"`

	// Raw is the complete, untrimmed response body ParseClaudeUsage was
	// given - this endpoint is undocumented and returns several fields
	// (feature-flag-looking names, an extra_usage/credits object) this type
	// doesn't name yet, so nothing is silently dropped for a future
	// ToDisplay to draw on.
	Raw json.RawMessage `json:"-"`

	DiagnosisFields
}

type claudeUsageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

// claudeCredentials is the shape of the Claude CLI's own credentials file
// (~/.claude/.credentials.json), not the usage API - unrelated to
// ClaudeUsage above.
type claudeCredentials struct {
	ClaudeAiOauth struct {
		AccessToken      string `json:"accessToken"`
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}
