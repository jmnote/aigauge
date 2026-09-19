package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrReauthenticationRequired marks a credential failure that retrying cannot
// repair, such as a revoked refresh token.
var ErrReauthenticationRequired = errors.New("reauthentication required")

var (
	flowMu      sync.Mutex
	activeFlows = map[string]*flowState{}
	refreshMu   sync.Mutex
	refreshes   = map[string]*refreshCall{}
)

type refreshCall struct {
	done chan struct{}
	tok  *Token
	body []byte
	err  error
}

type flowState struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// CancelActiveFlow cancels any in-progress OAuth browser flow.
func CancelActiveFlow() {
	flowMu.Lock()
	defer flowMu.Unlock()
	for key, flow := range activeFlows {
		flow.cancel()
		delete(activeFlows, key)
	}
}

// CancelActiveFlowAndWait cancels the current OAuth flow and waits for its
// loopback listener to be closed. This prevents a new flow from racing the
// cleanup of a listener on a fixed provider port.
func CancelActiveFlowAndWait() {
	flowMu.Lock()
	flows := make([]*flowState, 0, len(activeFlows))
	for key, flow := range activeFlows {
		flow.cancel()
		delete(activeFlows, key)
		flows = append(flows, flow)
	}
	flowMu.Unlock()
	for _, flow := range flows {
		waitForFlow(flow)
	}
}

// CancelAuthFlowAndWait cancels only the OAuth flow for tokenKey.
func CancelAuthFlowAndWait(tokenKey string) {
	flowMu.Lock()
	flow := activeFlows[tokenKey]
	if flow != nil {
		flow.cancel()
		delete(activeFlows, tokenKey)
	}
	flowMu.Unlock()
	waitForFlow(flow)
}

// cancelAndWait cancels flow (a no-op if nil) and waits up to 2s for it to
// finish unwinding, so its loopback listener is guaranteed closed before the
// caller reuses a fixed provider port.
func cancelAndWait(flow *flowState) {
	if flow != nil {
		flow.cancel()
	}
	waitForFlow(flow)
}

func waitForFlow(flow *flowState) {
	if flow == nil {
		return
	}
	select {
	case <-flow.done:
	case <-time.After(2 * time.Second):
	}
}

// StartAuthFlow initiates an OAuth 2.0 PKCE authentication flow. cfgType
// selects which provider's OAuth endpoints/client to use (e.g. "claude"); the
// resulting token is stored under tokenKey, which identifies the provider
// *instance* the user is connecting and may differ from cfgType once a type
// has more than one instance.
func StartAuthFlow(ctx context.Context, cfgType, tokenKey string, openBrowser func(string) error) (*Token, error) {
	cfg, ok := GetProviderConfig(cfgType)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", cfgType)
	}
	if cfg.AuthURL == "" {
		return nil, fmt.Errorf("browser login is not available for %s. Please log in to %s first, then connect in AI Gauge", cfg.Name, cfg.Name)
	}
	if cfg.ManualCode {
		return nil, fmt.Errorf("%s's login page does not redirect back to AI Gauge automatically - use BeginManualAuthFlow/CompleteManualAuthFlow instead", cfg.Name)
	}

	flowCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	flow := &flowState{cancel: cancel, done: make(chan struct{})}
	flowMu.Lock()
	prev := activeFlows[tokenKey]
	activeFlows[tokenKey] = flow
	flowMu.Unlock()
	// Wait for a preempted flow's loopback listener to close before binding a
	// new one - StartLoopbackServer below can otherwise race the old
	// listener's Close() on providers with a fixed RedirectPort.
	cancelAndWait(prev)

	defer func() {
		close(flow.done)
		flowMu.Lock()
		// A newer flow may have replaced this one while it was unwinding.
		// Do not clear the newer flow's cancellation handle.
		if activeFlows[tokenKey] == flow {
			delete(activeFlows, tokenKey)
		}
		flowMu.Unlock()
	}()

	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	var verifier, challenge string
	if cfg.UsePKCE {
		verifier, err = GenerateCodeVerifier()
		if err != nil {
			return nil, fmt.Errorf("failed to generate PKCE verifier: %w", err)
		}
		challenge = GenerateCodeChallenge(verifier)
	}

	server, err := StartLoopbackServer(state, cfg.RedirectPort, cfg.RedirectPath, cfg.RedirectHost)
	if err != nil {
		return nil, fmt.Errorf("failed to start loopback server: %w", err)
	}
	defer func() { _ = server.Close() }()

	redirectURI := server.CallbackURL()

	authURL, err := buildAuthorizeURL(cfg, redirectURI, state, challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth URL: %w", err)
	}

	if openBrowser != nil {
		if err := openBrowser(authURL); err != nil {
			return nil, fmt.Errorf("failed to launch browser: %w", err)
		}
	}

	code, err := server.WaitForCode(flowCtx)
	if err != nil {
		return nil, err
	}

	token, err := exchangeCodeForToken(flowCtx, cfg, code, verifier, redirectURI, state)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	if err := SaveToken(tokenKey, token); err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	return token, nil
}

func buildAuthorizeURL(cfg ProviderConfig, redirectURI, state, challenge string) (string, error) {
	u, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client_id", cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)

	if len(cfg.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	if challenge != "" {
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}

	for k, v := range cfg.CustomParams {
		q.Set(k, v)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

type tokenExchangeResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	SubscriptionType string `json:"subscription_type"`
	PlanType         string `json:"plan_type"`
	OrganizationUUID string `json:"organization_uuid"`
	OrganizationID   string `json:"organizationUuid"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type tokenEndpointError struct {
	statusCode int
	oauthCode  string
	message    string
}

func (e *tokenEndpointError) Error() string { return e.message }

// tokenHTTPClient is shared by exchangeCodeForToken and refreshTokenRaw so
// repeated token-endpoint calls (e.g. several provider instances refreshing
// at startup) can reuse keep-alive connections instead of each building its
// own throwaway client.
var tokenHTTPClient = &http.Client{Timeout: 15 * time.Second}

// postTokenRequest POSTs values to an OAuth token endpoint and returns the
// parsed response together with its raw body. exchangeCodeForToken and
// refreshTokenRaw share this request/response handling; only the values they
// post and what they do with a successful response differ.
func postTokenRequest(ctx context.Context, tokenURL string, values url.Values) (tokenExchangeResponse, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return tokenExchangeResponse{}, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := tokenHTTPClient.Do(req)
	if err != nil {
		return tokenExchangeResponse{}, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenExchangeResponse{}, nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var raw tokenExchangeResponse
		_ = json.Unmarshal(body, &raw)
		return tokenExchangeResponse{}, body, &tokenEndpointError{
			statusCode: resp.StatusCode,
			oauthCode:  raw.Error,
			message:    fmt.Sprintf("token endpoint returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	var raw tokenExchangeResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return tokenExchangeResponse{}, nil, fmt.Errorf("unmarshal token response: %w", err)
	}
	return raw, body, nil
}

func exchangeCodeForToken(ctx context.Context, cfg ProviderConfig, code, verifier, redirectURI, state string) (*Token, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", redirectURI)
	values.Set("client_id", cfg.ClientID)

	if cfg.ClientSecret != "" {
		values.Set("client_secret", cfg.ClientSecret)
	}
	if verifier != "" {
		values.Set("code_verifier", verifier)
	}
	// Claude's token endpoint expects the same state value the authorize
	// request sent back here too (confirmed against a real Claude Code CLI's
	// own token exchange request) - harmless to include for the loopback
	// providers as well, since an unrecognized form field is simply ignored.
	if state != "" {
		values.Set("state", state)
	}

	raw, _, err := postTokenRequest(ctx, cfg.TokenURL, values)
	if err != nil {
		return nil, err
	}

	if raw.Error != "" {
		desc := raw.ErrorDescription
		if desc == "" {
			desc = raw.Error
		}
		return nil, fmt.Errorf("oauth error: %s", desc)
	}

	if raw.AccessToken == "" {
		return nil, fmt.Errorf("no access_token in token response")
	}

	tok := &Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		Extra: Extra{
			Plan:             raw.SubscriptionType,
			OrganizationUUID: firstNonEmpty(raw.OrganizationUUID, raw.OrganizationID),
		},
	}
	if tok.Extra.Plan == "" {
		tok.Extra.Plan = raw.PlanType
	}
	if raw.ExpiresIn > 0 {
		tok.ExpiresAt = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}

	return tok, nil
}

// RefreshToken attempts to refresh an expired access token using the stored
// refresh token. cfgType selects the OAuth client/endpoints to refresh with;
// tokenKey identifies the provider instance whose stored token is refreshed.
func RefreshToken(ctx context.Context, cfgType, tokenKey string) (*Token, error) {
	tok, _, err := refreshToken(ctx, cfgType, tokenKey)
	return tok, err
}

// FetchRawTokenRefresh refreshes tokenKey's token exactly as RefreshToken
// does (including persisting the result, so the locally stored credential
// stays in sync with whatever the provider issued - some providers rotate
// the refresh token itself, invalidating the old one server-side), but also
// returns the unconverted token endpoint response body. Used by
// hack/fixtures/fixtures.go tokens to capture the real response shape (e.g.
// whether a provider's token includes an id_token to identify the account
// by) for fixture development.
func FetchRawTokenRefresh(ctx context.Context, cfgType, tokenKey string) ([]byte, error) {
	_, body, err := refreshToken(ctx, cfgType, tokenKey)
	return body, err
}

// refreshToken coalesces concurrent refreshes for one provider instance. This
// prevents refresh-token rotation from making the second request fail with
// invalid_grant after the first request has already succeeded.
func refreshToken(ctx context.Context, cfgType, tokenKey string) (*Token, []byte, error) {
	refreshMu.Lock()
	if call := refreshes[tokenKey]; call != nil {
		refreshMu.Unlock()
		select {
		case <-call.done:
			return call.tok, call.body, call.err
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	call := &refreshCall{done: make(chan struct{})}
	refreshes[tokenKey] = call
	refreshMu.Unlock()

	call.tok, call.body, call.err = refreshTokenRaw(ctx, cfgType, tokenKey)
	refreshMu.Lock()
	delete(refreshes, tokenKey)
	close(call.done)
	refreshMu.Unlock()
	return call.tok, call.body, call.err
}

func refreshTokenRaw(ctx context.Context, cfgType, tokenKey string) (*Token, []byte, error) {
	tok, err := GetToken(tokenKey)
	if err != nil || tok == nil {
		return nil, nil, fmt.Errorf("no stored token for %q", tokenKey)
	}
	if tok.RefreshToken == "" {
		return nil, nil, fmt.Errorf("%w: no refresh token available for %q", ErrReauthenticationRequired, tokenKey)
	}

	cfg, ok := GetProviderConfig(cfgType)
	if !ok {
		return nil, nil, fmt.Errorf("unknown provider %q", cfgType)
	}

	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", tok.RefreshToken)
	values.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		values.Set("client_secret", cfg.ClientSecret)
	}

	raw, body, err := postTokenRequest(ctx, cfg.TokenURL, values)
	if err != nil {
		var endpointErr *tokenEndpointError
		if errors.As(err, &endpointErr) && (endpointErr.statusCode == http.StatusUnauthorized || endpointErr.statusCode == http.StatusForbidden || endpointErr.oauthCode == "invalid_grant") {
			return nil, body, fmt.Errorf("%w: %v", ErrReauthenticationRequired, err)
		}
		return nil, body, err
	}
	if raw.Error != "" {
		if raw.Error == "invalid_grant" {
			return nil, body, fmt.Errorf("%w: refresh response contained oauth error: %s", ErrReauthenticationRequired, raw.Error)
		}
		return nil, body, fmt.Errorf("refresh response contained oauth error: %s", raw.Error)
	}

	if raw.AccessToken == "" {
		return nil, nil, fmt.Errorf("refresh response had no access_token")
	}

	tok.AccessToken = raw.AccessToken
	if raw.RefreshToken != "" {
		tok.RefreshToken = raw.RefreshToken
	}
	if raw.ExpiresIn > 0 {
		tok.ExpiresAt = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	if organizationUUID := firstNonEmpty(raw.OrganizationUUID, raw.OrganizationID); organizationUUID != "" {
		tok.Extra.OrganizationUUID = organizationUUID
	}

	if err := SaveToken(tokenKey, tok); err != nil {
		return nil, nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	return tok, body, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// GetValidAccessToken returns an unexpired access token, automatically
// refreshing it (using cfgType's OAuth client) if needed.
func GetValidAccessToken(ctx context.Context, cfgType, tokenKey string) (string, error) {
	tok, err := GetToken(tokenKey)
	if err != nil || tok == nil || tok.AccessToken == "" {
		return "", fmt.Errorf("not authenticated with %s", tokenKey)
	}

	if tok.IsExpired() {
		if tok.RefreshToken != "" {
			refreshed, refreshErr := RefreshToken(ctx, cfgType, tokenKey)
			if refreshErr == nil && refreshed != nil {
				return refreshed.AccessToken, nil
			}
			// During the two-minute early-refresh window the existing access
			// token can still be used. Once it has truly expired, returning it
			// would turn a refresh outage into a misleading 401/login prompt.
			if !tok.ExpiresAt.IsZero() && time.Now().Before(tok.ExpiresAt) {
				return tok.AccessToken, nil
			}
			return "", fmt.Errorf("failed to refresh access token for %q: %w", tokenKey, refreshErr)
		}
		return "", fmt.Errorf("%w: access token for %q is expired and has no refresh token", ErrReauthenticationRequired, tokenKey)
	}

	return tok.AccessToken, nil
}

// manualCodeRedirectURI is where a ManualCode provider's authorize page
// redirects to - a page it hosts itself, not a loopback address, so it can
// display the code for the user to copy instead of a local server capturing
// it. It is only meaningful for providers with ManualCode set.
const manualCodeRedirectURI = "https://platform.claude.com/oauth/code/callback"

type pendingManualFlow struct {
	cfgType  string
	verifier string
	state    string
	inFlight bool
	ctx      context.Context
	cancel   context.CancelFunc
}

var (
	manualFlowMu sync.Mutex
	manualFlows  = map[string]*pendingManualFlow{} // keyed by tokenKey
)

// BeginManualAuthFlow builds the authorize URL for a ManualCode provider
// (see ProviderConfig.ManualCode) and remembers the PKCE verifier/state
// needed to complete it once the user pastes back the code shown on the
// provider's own redirect page. The caller is expected to open authURL in
// the system browser.
func BeginManualAuthFlow(cfgType, tokenKey string) (authURL string, err error) {
	cfg, ok := GetProviderConfig(cfgType)
	if !ok {
		return "", fmt.Errorf("unknown provider %q", cfgType)
	}
	if !cfg.ManualCode {
		return "", fmt.Errorf("%s does not use the manual code flow", cfg.Name)
	}

	state, err := GenerateState()
	if err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	challenge := GenerateCodeChallenge(verifier)

	authURL, err = buildAuthorizeURL(cfg, manualCodeRedirectURI, state, challenge)
	if err != nil {
		return "", fmt.Errorf("failed to build auth URL: %w", err)
	}

	flowCtx, cancel := context.WithCancel(context.Background())
	manualFlowMu.Lock()
	if previous := manualFlows[tokenKey]; previous != nil {
		previous.cancel()
	}
	manualFlows[tokenKey] = &pendingManualFlow{cfgType: cfgType, verifier: verifier, state: state, ctx: flowCtx, cancel: cancel}
	manualFlowMu.Unlock()

	return authURL, nil
}

// CancelManualAuthFlow discards a pending manual-code login for tokenKey.
// The browser page cannot be closed by the app, but its verifier/state should
// not survive after the provider instance is removed or the dialog is closed.
func CancelManualAuthFlow(tokenKey string) {
	manualFlowMu.Lock()
	if pending := manualFlows[tokenKey]; pending != nil {
		pending.cancel()
		delete(manualFlows, tokenKey)
	}
	manualFlowMu.Unlock()
}

// CompleteManualAuthFlow exchanges the code the user pasted back for a
// token, using the PKCE verifier BeginManualAuthFlow generated for tokenKey.
// rawCode must be "CODE#STATE", the shape the provider's redirect page
// displays; the state is always checked against the one BeginManualAuthFlow
// generated, the same CSRF protection the loopback flow gets from matching
// state in the redirect query string.
func CompleteManualAuthFlow(tokenKey, rawCode string) (*Token, error) {
	manualFlowMu.Lock()
	pending, ok := manualFlows[tokenKey]
	if ok && pending.inFlight {
		manualFlowMu.Unlock()
		return nil, fmt.Errorf("authentication is already being completed for %q", tokenKey)
	}
	if ok {
		pending.inFlight = true
	}
	manualFlowMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no authentication in progress for %q - click Connect again", tokenKey)
	}
	failed := true
	defer func() {
		manualFlowMu.Lock()
		if current := manualFlows[tokenKey]; current == pending {
			if failed {
				current.inFlight = false
			} else {
				delete(manualFlows, tokenKey)
			}
		}
		manualFlowMu.Unlock()
	}()

	code := strings.TrimSpace(rawCode)
	idx := strings.Index(code, "#")
	if idx < 0 {
		return nil, fmt.Errorf("the pasted text is missing its #state suffix - copy the full code shown on the page and try again")
	}
	pastedState := code[idx+1:]
	code = code[:idx]
	if pastedState != pending.state {
		return nil, fmt.Errorf("the pasted code does not match this login attempt - click Connect and try again")
	}
	if code == "" {
		return nil, fmt.Errorf("no code was pasted")
	}

	cfg, ok := GetProviderConfig(pending.cfgType)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", pending.cfgType)
	}

	ctx, cancel := context.WithTimeout(pending.ctx, 30*time.Second)
	defer cancel()
	token, err := exchangeCodeForToken(ctx, cfg, code, pending.verifier, manualCodeRedirectURI, pending.state)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Serialize the final liveness check and save with cancellation. This
	// prevents RemoveProviderInstance from deleting a token immediately before
	// this completion stores it again.
	manualFlowMu.Lock()
	if manualFlows[tokenKey] != pending {
		manualFlowMu.Unlock()
		return nil, fmt.Errorf("authentication was cancelled for %q", tokenKey)
	}
	if err := SaveToken(tokenKey, token); err != nil {
		manualFlowMu.Unlock()
		return nil, fmt.Errorf("failed to store token: %w", err)
	}
	manualFlowMu.Unlock()
	failed = false
	return token, nil
}
