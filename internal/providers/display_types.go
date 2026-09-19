package providers

// DisplayUsage is the common shape every provider's usage converts to
// before reaching the frontend: one renderer handles all three providers
// instead of one per provider, and every provider-specific quirk (Codex's
// relative-seconds resets, Antigravity's window sort order/labels) is
// resolved once, here, in Go.
type DisplayUsage struct {
	Plan         string              `json:"plan"`
	User         string              `json:"user"`
	Email        string              `json:"email,omitempty"`
	DisplayName  string              `json:"displayName,omitempty"`
	ResetCredits *int                `json:"resetCredits,omitempty"`
	Groups       []DisplayUsageGroup `json:"groups"`
	FetchedAt    string              `json:"fetchedAt"`

	DiagnosisFields
}

// DisplayUsageGroup is one titled section of buckets (e.g. Antigravity's
// "Gemini Models"). Name is empty for a provider with no natural grouping
// (Codex, Claude), which the frontend renders as a flat list with no title.
type DisplayUsageGroup struct {
	Name    string               `json:"name"`
	Buckets []DisplayUsageBucket `json:"buckets"`
}

// DisplayUsageBucket is one progress row: Label and ResetTime are always
// display-ready (an absolute timestamp, already sorted/labeled), unlike the
// provider-specific bucket types this converts from. Detail carries optional
// supplementary count text, such as "84/200" for Copilot.
type DisplayUsageBucket struct {
	Label     string  `json:"label"`
	Detail    string  `json:"detail,omitempty"`
	Remaining float64 `json:"remaining"`
	ResetTime string  `json:"resetTime"`
}
