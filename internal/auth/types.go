package auth

import (
	"sync"
	"time"
)

// Token represents an OAuth 2.0 token response stored securely by AI Gauge.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Extra        Extra     `json:"extra,omitempty"`
}

// Extra contains provider-specific token metadata that is not part of the
// OAuth token response itself.
type Extra struct {
	Plan               string `json:"plan,omitempty"`
	Email              string `json:"email,omitempty"`
	AccountDisplayName string `json:"accountDisplayName,omitempty"`
	OrganizationUUID   string `json:"organizationUuid,omitempty"`
}

// IsExpired reports whether the token is expired or close to expiration (within 2 minutes).
func (t *Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(2 * time.Minute).After(t.ExpiresAt)
}

// ProviderConfig defines OAuth endpoints and client settings for a provider.
// Only Claude and Codex are configured here - see the comment below
// DefaultConfigs for why Antigravity deliberately has no entry.
//
// Both entries reuse the provider's own official CLI's public OAuth client
// (client_id) rather than a client AI Gauge registers itself. That is
// deliberate: these providers' usage/quota APIs are largely undocumented, so
// there is no "properly register our own app" path to the data - only
// reusing the same client the official CLI already uses, the same way every
// other third-party usage-tracker for these tools does.
type ProviderConfig struct {
	ID           string
	Name         string
	AuthURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	UsePKCE      bool
	CustomParams map[string]string

	// RedirectPort/RedirectPath/RedirectHost fix the loopback callback
	// address for providers whose OAuth client only has that exact
	// redirect_uri registered - since we're reusing each provider's own
	// client rather than one we registered ourselves, we don't get to pick
	// our own port (or, for Codex, even our own loopback hostname: its
	// authorize endpoint checks redirect_uri as an exact string match
	// against "http://localhost:1455/...", so "http://127.0.0.1:1455/..."
	// - otherwise equivalent - is rejected as an invalid authorize request).
	// RedirectPort zero means "any free port"; RedirectHost empty means
	// "127.0.0.1" (StartLoopbackServer's original behavior).
	RedirectPort int
	RedirectPath string
	RedirectHost string

	// ManualCode is true for a provider whose OAuth client's redirect_uri is
	// a page it hosts itself rather than a loopback address - Claude's
	// authorization redirects to platform.claude.com, which displays a
	// CODE#STATE string for the user to copy and paste back into AI Gauge,
	// since a local server never sees that redirect at all.
	ManualCode bool
}

// DefaultConfigs holds the default OAuth configurations for each supported provider.
var DefaultConfigs = map[string]ProviderConfig{
	// Claude's endpoints and scopes here are exactly what the real, currently
	// installed Claude Code CLI requests (confirmed by extracting the
	// BASE_API_URL/CLIENT_ID/TOKEN_URL/MANUAL_REDIRECT_URL constants from its
	// own binary) - Anthropic has since moved this flow from
	// console.anthropic.com to platform.claude.com, which is why an earlier
	// version of this config (still pointed at console.anthropic.com) got the
	// user logged into claude.ai but never reached the page that displays the
	// code, since that redirect_uri was no longer the one the client is
	// configured for.
	"claude": {
		ID:       "claude",
		Name:     "Claude",
		AuthURL:  "https://claude.ai/oauth/authorize",
		TokenURL: "https://platform.claude.com/v1/oauth/token",
		ClientID: "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		Scopes: []string{
			"user:inference", "user:profile", "user:sessions:claude_code", "user:mcp_servers",
		},
		UsePKCE:    true,
		ManualCode: true,
		CustomParams: map[string]string{
			"code": "true",
		},
	},
	"codex": {
		ID:           "codex",
		Name:         "Codex",
		AuthURL:      "https://auth.openai.com/oauth/authorize",
		TokenURL:     "https://auth.openai.com/oauth/token",
		ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
		Scopes:       []string{"openid", "profile", "email", "offline_access"},
		UsePKCE:      true,
		RedirectPort: 1455,
		RedirectPath: "/auth/callback",
		RedirectHost: "localhost",
		CustomParams: map[string]string{
			"id_token_add_organizations": "true",
			"codex_cli_simplified_flow":  "true",
			// The authorize endpoint validates originator against a fixed set
			// of known CLI clients and rejects anything else with
			// "invalid_authorize_request" - it must be the real Codex CLI's
			// value ("codex_cli_rs", confirmed against the CLI's own source at
			// codex-rs/login/src/server.rs), not an app-identifying value of
			// our own like "aigauge".
			"originator": "codex_cli_rs",
		},
	},
	// Antigravity deliberately has no entry here: unlike Claude/Codex, its
	// usage lookup goes through the locally installed agy CLI (see
	// internal/providers/antigravity.go), not a token AI Gauge holds itself.
	// Google's terms treat logging into Antigravity through a third-party
	// tool - which includes reusing its OAuth client to authenticate outside
	// the official CLI, or reading its local token store to call the API
	// directly - as a violation, so this app never does either for
	// Antigravity.
}

var (
	configMu sync.RWMutex
	configs  = map[string]ProviderConfig{}
)

func init() {
	for k, v := range DefaultConfigs {
		configs[k] = v
	}
}

// GetProviderConfig returns the configuration for the requested provider ID.
func GetProviderConfig(id string) (ProviderConfig, bool) {
	configMu.RLock()
	defer configMu.RUnlock()
	cfg, ok := configs[id]
	return cfg, ok
}

// SetProviderConfig allows overriding or setting a custom provider configuration.
func SetProviderConfig(id string, cfg ProviderConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	configs[id] = cfg
}
