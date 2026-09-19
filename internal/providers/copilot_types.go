package providers

import "encoding/json"

// CopilotUsage mirrors https://api.github.com/copilot_internal/user's response
// format for quota and plan information, along with optional GitHub Actions
// and Codespaces billing metrics.
type CopilotUsage struct {
	CopilotPlan    string                          `json:"copilot_plan"`
	QuotaResetDate string                          `json:"quota_reset_date"`
	QuotaSnapshots map[string]CopilotQuotaSnapshot `json:"quota_snapshots"`
	FetchedAt      string                          `json:"fetchedAt"`

	// Raw contains the untrimmed response body ParseCopilotUsage received.
	Raw json.RawMessage `json:"-"`

	DiagnosisFields
}

// CopilotQuotaSnapshot represents the quota consumption state for an individual
// feature (e.g. premium_interactions, chat, completions).
type CopilotQuotaSnapshot struct {
	Entitlement      float64 `json:"entitlement"`
	Remaining        float64 `json:"remaining"`
	PercentRemaining float64 `json:"percent_remaining"`
	Unlimited        bool    `json:"unlimited"`
}

// copilotAuth is the stored credential shape for GitHub Copilot.
type copilotAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}
