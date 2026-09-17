package providers

import "encoding/json"

// CodexUsage mirrors https://chatgpt.com/backend-api/wham/usage's response
// field-for-field - this is what a raw hack/fixtures/usage/usage_codex_*.json
// fixture looks like. FetchedAt isn't part of that response; it's stamped
// on after the fact (the local clock, at fetch time) so this type carries
// the one piece of bookkeeping ToDisplay needs. ToDisplay (codex.go) does
// all validation and unit conversion; this type does none.
type CodexUsage struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow struct {
			UsedPercent       *float64 `json:"used_percent"`
			ResetAfterSeconds *int     `json:"reset_after_seconds"`
		} `json:"primary_window"`
		SecondaryWindow struct {
			UsedPercent       *float64 `json:"used_percent"`
			ResetAfterSeconds *int     `json:"reset_after_seconds"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
	FetchedAt string `json:"fetchedAt"`

	// Raw is the complete, untrimmed response body ParseCodexUsage was
	// given - this endpoint is undocumented and returns several fields
	// (credits, model_usage, promo, ...) this type doesn't name yet, so
	// nothing is silently dropped for a future ToDisplay to draw on.
	Raw json.RawMessage `json:"-"`

	DiagnosisFields
}

// codexAuth is the shape of the Codex CLI's own credentials file
// (~/.codex/auth.json), not the usage API - unrelated to CodexUsage above.
type codexAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}
