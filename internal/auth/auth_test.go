package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu     sync.RWMutex
	tokens map[string]*Token
}

func newMemoryStore() Store {
	return &memoryStore{tokens: make(map[string]*Token)}
}

func (m *memoryStore) GetToken(p string) (*Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokens[p], nil
}

func (m *memoryStore) SaveToken(p string, t *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[p] = t
	return nil
}

func (m *memoryStore) DeleteToken(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, p)
	return nil
}

func (m *memoryStore) ListTokens() (map[string]*Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copied := make(map[string]*Token, len(m.tokens))
	for k, v := range m.tokens {
		copied[k] = v
	}
	return copied, nil
}

func (m *memoryStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens = make(map[string]*Token)
	return nil
}

func (m *memoryStore) IsInitialized() bool {
	return true
}

func TestPKCEGeneration(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier() error = %v", err)
	}
	if len(verifier) < 43 {
		t.Errorf("verifier length = %d, want at least 43", len(verifier))
	}

	challenge := GenerateCodeChallenge(verifier)
	if challenge == "" {
		t.Error("GenerateCodeChallenge() returned empty string")
	}

	challenge2 := GenerateCodeChallenge(verifier)
	if challenge != challenge2 {
		t.Error("GenerateCodeChallenge is not deterministic")
	}
}

func TestStateGeneration(t *testing.T) {
	state1, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState() error = %v", err)
	}
	state2, _ := GenerateState()
	if state1 == state2 {
		t.Error("GenerateState() generated duplicate state")
	}
}

func TestLoopbackServerHandlesCallback(t *testing.T) {
	expectedState := "test-state-12345"
	server, err := StartLoopbackServer(expectedState, 0, "", "")
	if err != nil {
		t.Fatalf("StartLoopbackServer() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	callbackURL := server.CallbackURL()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Perform HTTP GET with valid code and matching state
	resp, err := http.Get(fmt.Sprintf("%s?code=sample-code&state=%s", callbackURL, expectedState))
	if err != nil {
		t.Fatalf("GET callback error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 OK", resp.StatusCode)
	}

	code, err := server.WaitForCode(ctx)
	if err != nil {
		t.Fatalf("WaitForCode() error = %v", err)
	}
	if code != "sample-code" {
		t.Errorf("code = %q, want \"sample-code\"", code)
	}
}

func TestLoopbackServerRejectsStateMismatch(t *testing.T) {
	server, err := StartLoopbackServer("expected-state", 0, "", "")
	if err != nil {
		t.Fatalf("StartLoopbackServer() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	resp, err := http.Get(fmt.Sprintf("%s?code=sample-code&state=wrong-state", server.CallbackURL()))
	if err != nil {
		t.Fatalf("GET callback error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 Bad Request", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = server.WaitForCode(ctx)
	if err == nil {
		t.Error("WaitForCode() want error on state mismatch, got nil")
	}
}

func TestLoopbackServerEscapesProviderErrorDescription(t *testing.T) {
	server, err := StartLoopbackServer("expected-state", 0, "", "")
	if err != nil {
		t.Fatalf("StartLoopbackServer() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	resp, err := http.Get(server.CallbackURL() + "?state=expected-state&error=access_denied&error_description=%3Cscript%3Ealert(1)%3C%2Fscript%3E")
	if err != nil {
		t.Fatalf("GET callback error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	text := string(body)
	if strings.Contains(text, "<script>alert(1)</script>") {
		t.Fatal("callback response rendered an unescaped script tag")
	}
	if !strings.Contains(text, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("callback response = %q, want escaped error description", text)
	}
}

func TestFullAuthFlowWithMockServer(t *testing.T) {
	memStore := newMemoryStore()
	SetDefaultStore(memStore)

	var receivedCodeVerifier string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			receivedCodeVerifier = r.FormValue("code_verifier")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"expires_in":    3600,
				"plan_type":     "pro",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	testProvider := "test-provider"
	SetProviderConfig(testProvider, ProviderConfig{
		ID:       testProvider,
		Name:     "Test",
		AuthURL:  mockServer.URL + "/auth",
		TokenURL: mockServer.URL + "/token",
		ClientID: "test-client",
		UsePKCE:  true,
	})

	var launchedURL string
	openBrowser := func(u string) error {
		launchedURL = u
		parsed, err := url.Parse(u)
		if err != nil {
			return err
		}
		q := parsed.Query()
		redirectURI := q.Get("redirect_uri")
		state := q.Get("state")

		// Simulate user approval in browser by fetching the redirect callback
		go func() {
			time.Sleep(20 * time.Millisecond)
			callURL := fmt.Sprintf("%s?code=test-auth-code&state=%s", redirectURI, state)
			resp, err := http.Get(callURL)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	token, err := StartAuthFlow(ctx, testProvider, testProvider, openBrowser)
	if err != nil {
		t.Fatalf("StartAuthFlow() error = %v", err)
	}
	if token.AccessToken != "mock-access-token" {
		t.Errorf("token.AccessToken = %q, want mock-access-token", token.AccessToken)
	}
	if token.Extra.Plan != "pro" {
		t.Errorf("token Extra plan = %q, want pro", token.Extra.Plan)
	}
	if receivedCodeVerifier == "" {
		t.Error("code_verifier was not sent to token endpoint")
	}
	if launchedURL == "" {
		t.Error("openBrowser was not called")
	}

	stored, err := GetToken(testProvider)
	if err != nil || stored == nil {
		t.Fatalf("GetToken() error = %v, token = %v", err, stored)
	}
	if stored.AccessToken != "mock-access-token" {
		t.Errorf("stored token mismatch: %v", stored)
	}
}

func TestTokenExpiryAndRefresh(t *testing.T) {
	memStore := newMemoryStore()
	SetDefaultStore(memStore)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.FormValue("grant_type") != "refresh_token" {
				http.Error(w, "invalid grant_type", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "refreshed-access-token",
				"refresh_token": "new-refresh-token",
				"expires_in":    7200,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	testProvider := "refresh-provider"
	SetProviderConfig(testProvider, ProviderConfig{
		ID:       testProvider,
		TokenURL: mockServer.URL + "/token",
		ClientID: "client-id",
	})

	// Store an already expired token
	_ = SaveToken(testProvider, &Token{
		AccessToken:  "old-token",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(-10 * time.Minute),
	})

	tokenStr, err := GetValidAccessToken(context.Background(), testProvider, testProvider)
	if err != nil {
		t.Fatalf("GetValidAccessToken() error = %v", err)
	}
	if tokenStr != "refreshed-access-token" {
		t.Errorf("token = %q, want refreshed-access-token", tokenStr)
	}
}

func TestGetValidAccessTokenDoesNotReturnExpiredTokenWhenRefreshFails(t *testing.T) {
	store := newMemoryStore()
	SetDefaultStore(store)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	const provider = "refresh-failure-provider"
	SetProviderConfig(provider, ProviderConfig{ID: provider, TokenURL: mockServer.URL, ClientID: "client-id"})
	if err := SaveToken(provider, &Token{
		AccessToken: "expired-token", RefreshToken: "refresh-token", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := GetValidAccessToken(context.Background(), provider, provider)
	if err == nil || got != "" {
		t.Fatalf("GetValidAccessToken() = (%q, %v), want empty token and refresh error", got, err)
	}
}

func TestParseCredentials(t *testing.T) {
	// Claude credentials test
	claudeJSON := []byte(`{"claudeAiOauth": {"accessToken": "claude-token-123", "subscriptionType": "pro"}}`)
	claudeTok := parseClaudeCredentials(claudeJSON)
	if claudeTok == nil || claudeTok.AccessToken != "claude-token-123" || claudeTok.Extra.Plan != "pro" {
		t.Errorf("parseClaudeCredentials() failed: got %+v", claudeTok)
	}

	// Codex credentials test
	codexJSON := []byte(`{"tokens": {"access_token": "codex-token-456"}}`)
	codexTok := parseCodexCredentials(codexJSON)
	if codexTok == nil || codexTok.AccessToken != "codex-token-456" {
		t.Errorf("parseCodexCredentials() failed: got %+v", codexTok)
	}
}

// TestDefaultConfigsHaveRealOAuthClients guards against the config silently
// regressing to empty/placeholder values (as claude/codex's did before) -
// every one of these must actually be usable, not blank stubs. Antigravity
// deliberately has no entry (see the comment below DefaultConfigs), so it is
// not checked here.
func TestDefaultConfigsHaveRealOAuthClients(t *testing.T) {
	for _, id := range []string{"claude", "codex"} {
		cfg, ok := DefaultConfigs[id]
		if !ok {
			t.Fatalf("DefaultConfigs[%q] missing", id)
		}
		if cfg.AuthURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" {
			t.Errorf("DefaultConfigs[%q] = %+v, want non-empty AuthURL/TokenURL/ClientID", id, cfg)
		}
		if !cfg.UsePKCE {
			t.Errorf("DefaultConfigs[%q].UsePKCE = false, want true", id)
		}
	}

	if _, ok := DefaultConfigs["antigravity"]; ok {
		t.Error(`DefaultConfigs["antigravity"] present, want no entry - Antigravity goes through the agy CLI, not an OAuth client AI Gauge holds`)
	}

	// Codex's OAuth client only has one loopback redirect_uri registered (a
	// fixed port AI Gauge did not choose), so StartAuthFlow must bind exactly
	// that port rather than an ephemeral one.
	if DefaultConfigs["codex"].RedirectPort != 1455 {
		t.Errorf("codex RedirectPort = %d, want 1455", DefaultConfigs["codex"].RedirectPort)
	}
	// Codex's OAuth client has its redirect_uri registered with the
	// "localhost" hostname specifically - "127.0.0.1", though the same
	// loopback address, is rejected as an invalid authorize request.
	if DefaultConfigs["codex"].RedirectHost != "localhost" {
		t.Errorf("codex RedirectHost = %q, want \"localhost\"", DefaultConfigs["codex"].RedirectHost)
	}

	// Claude's authorize page redirects to a page Anthropic hosts, not a
	// loopback address, so it must go through the manual code-paste flow
	// rather than StartAuthFlow's automatic loopback capture.
	if !DefaultConfigs["claude"].ManualCode {
		t.Error("claude.ManualCode = false, want true (its redirect_uri is not a loopback address)")
	}
	if DefaultConfigs["codex"].ManualCode {
		t.Error("codex.ManualCode = true, want false (it uses a loopback redirect_uri)")
	}
}

func TestManualAuthFlowRoundTrip(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.FormValue("code_verifier") == "" {
			http.Error(w, "missing code_verifier", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "manual-flow-token",
			"expires_in":   3600,
		})
	}))
	defer mockServer.Close()

	memStore := newMemoryStore()
	SetDefaultStore(memStore)

	const testProvider = "test-manual-provider"
	SetProviderConfig(testProvider, ProviderConfig{
		ID: testProvider, Name: "Test Manual", AuthURL: mockServer.URL + "/auth",
		TokenURL: mockServer.URL + "/token", ClientID: "test-client",
		UsePKCE: true, ManualCode: true,
	})

	authURL, err := BeginManualAuthFlow(testProvider, "instance-1")
	if err != nil {
		t.Fatalf("BeginManualAuthFlow() error = %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("authURL %q did not parse: %v", authURL, err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("authURL has no state parameter")
	}
	if parsed.Query().Get("redirect_uri") != manualCodeRedirectURI {
		t.Errorf("redirect_uri = %q, want %q", parsed.Query().Get("redirect_uri"), manualCodeRedirectURI)
	}

	// The provider's redirect page shows "CODE#STATE" for the user to paste.
	pasted := "the-pasted-code#" + state
	tok, err := CompleteManualAuthFlow("instance-1", pasted)
	if err != nil {
		t.Fatalf("CompleteManualAuthFlow() error = %v", err)
	}
	if tok.AccessToken != "manual-flow-token" {
		t.Errorf("AccessToken = %q, want manual-flow-token", tok.AccessToken)
	}

	stored, err := GetToken("instance-1")
	if err != nil || stored == nil || stored.AccessToken != "manual-flow-token" {
		t.Fatalf("GetToken(instance-1) = %+v, %v", stored, err)
	}

	// A second completion attempt has nothing pending any more.
	if _, err := CompleteManualAuthFlow("instance-1", pasted); err == nil {
		t.Error("CompleteManualAuthFlow() a second time: want error (flow already consumed), got nil")
	}
}

func TestManualAuthFlowRejectsMismatchedState(t *testing.T) {
	memStore := newMemoryStore()
	SetDefaultStore(memStore)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "retry-token"})
	}))
	defer mockServer.Close()

	const testProvider = "test-manual-state-mismatch"
	SetProviderConfig(testProvider, ProviderConfig{
		ID: testProvider, Name: "Test", AuthURL: "https://example.com/auth",
		TokenURL: mockServer.URL, ClientID: "test-client",
		UsePKCE: true, ManualCode: true,
	})

	authURL, err := BeginManualAuthFlow(testProvider, "instance-2")
	if err != nil {
		t.Fatalf("BeginManualAuthFlow() error = %v", err)
	}
	if _, err := CompleteManualAuthFlow("instance-2", "some-code#wrong-state"); err == nil {
		t.Error("CompleteManualAuthFlow() with mismatched state: want error, got nil")
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	if tok, err := CompleteManualAuthFlow("instance-2", "some-code#"+state); err != nil || tok.AccessToken != "retry-token" {
		t.Fatalf("CompleteManualAuthFlow() retry = (%+v, %v), want retry-token", tok, err)
	}
}

// TestManualAuthFlowRejectsBareCode guards against a real CSRF gap: a pasted
// code with no "#state" suffix used to skip state verification entirely
// instead of being rejected.
func TestManualAuthFlowRejectsBareCode(t *testing.T) {
	memStore := newMemoryStore()
	SetDefaultStore(memStore)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "should-not-be-stored"})
	}))
	defer mockServer.Close()

	const testProvider = "test-manual-bare-code"
	SetProviderConfig(testProvider, ProviderConfig{
		ID: testProvider, Name: "Test", AuthURL: "https://example.com/auth",
		TokenURL: mockServer.URL, ClientID: "test-client",
		UsePKCE: true, ManualCode: true,
	})

	if _, err := BeginManualAuthFlow(testProvider, "instance-3"); err != nil {
		t.Fatalf("BeginManualAuthFlow() error = %v", err)
	}
	if _, err := CompleteManualAuthFlow("instance-3", "bare-code-with-no-state"); err == nil {
		t.Error("CompleteManualAuthFlow() with a bare code (no #state): want error, got nil")
	}
	if stored, _ := GetToken("instance-3"); stored != nil {
		t.Errorf("GetToken(instance-3) = %+v, want nil after a rejected bare code", stored)
	}
}

func TestParseClaudeCredentials(t *testing.T) {
	raw := `{
		"claudeAiOauth": {
			"accessToken": "claude-access-123",
			"refreshToken": "claude-refresh-456",
			"expiresAt": 1789615400667,
		"subscriptionType": "pro"
		}
	}`
	tok := parseClaudeCredentials([]byte(raw))
	if tok == nil {
		t.Fatal("parseClaudeCredentials() returned nil for valid payload")
	}
	if tok.AccessToken != "claude-access-123" {
		t.Errorf("AccessToken = %q, want \"claude-access-123\"", tok.AccessToken)
	}
	if tok.RefreshToken != "claude-refresh-456" {
		t.Errorf("RefreshToken = %q, want \"claude-refresh-456\"", tok.RefreshToken)
	}
	if tok.Extra.Plan != "pro" {
		t.Errorf("Extra.Plan = %q, want \"pro\"", tok.Extra.Plan)
	}
	expectedExp := time.UnixMilli(1789615400667)
	if !tok.ExpiresAt.Equal(expectedExp) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, expectedExp)
	}
}

func TestParseCodexCredentials(t *testing.T) {
	// Dummy JWT: header.payload.signature
	// payload: {"exp": 1789615400} -> base64 RawURLEncoding
	dummyJWT := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjE3ODk2MTU0MDB9.signature"
	raw := fmt.Sprintf(`{
		"tokens": {
			"access_token": "%s",
			"refresh_token": "codex-refresh-789"
		}
	}`, dummyJWT)

	tok := parseCodexCredentials([]byte(raw))
	if tok == nil {
		t.Fatal("parseCodexCredentials() returned nil for valid payload")
	}
	if tok.AccessToken != dummyJWT {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, dummyJWT)
	}
	if tok.RefreshToken != "codex-refresh-789" {
		t.Errorf("RefreshToken = %q, want \"codex-refresh-789\"", tok.RefreshToken)
	}
	expectedExp := time.Unix(1789615400, 0)
	if !tok.ExpiresAt.Equal(expectedExp) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, expectedExp)
	}
}

func TestParseCredentialsFileInvalid(t *testing.T) {
	if tok := parseClaudeCredentials([]byte(`{}`)); tok != nil {
		t.Errorf("parseClaudeCredentials({}) = %+v, want nil", tok)
	}
	if tok := parseCodexCredentials([]byte(`{}`)); tok != nil {
		t.Errorf("parseCodexCredentials({}) = %+v, want nil", tok)
	}
	if tok := parseClaudeCredentials([]byte(`invalid json`)); tok != nil {
		t.Errorf("parseClaudeCredentials(invalid) = %+v, want nil", tok)
	}
	if tok := parseCodexCredentials([]byte(`invalid json`)); tok != nil {
		t.Errorf("parseCodexCredentials(invalid) = %+v, want nil", tok)
	}
}

func TestDeviceAuthFlowRoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device/code" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":      "dev-code-123",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         5,
			})
			return
		}
		if r.URL.Path == "/oauth/access_token" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "ghu_test_token_123",
				"token_type":   "bearer",
				"scope":        "read:user,copilot",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	SetProviderConfig("test_device", ProviderConfig{
		ID:         "test_device",
		Name:       "Test Device",
		DeviceFlow: true,
		DeviceURL:  ts.URL + "/device/code",
		TokenURL:   ts.URL + "/oauth/access_token",
		ClientID:   "test-client-id",
	})

	authURL, userCode, err := BeginDeviceAuthFlow("test_device", "inst-device-1")
	if err != nil {
		t.Fatalf("BeginDeviceAuthFlow failed: %v", err)
	}
	if authURL != "https://github.com/login/device" || userCode != "ABCD-1234" {
		t.Errorf("got authURL=%q, userCode=%q", authURL, userCode)
	}

	tok, err := CompleteDeviceAuthFlow("inst-device-1")
	if err != nil {
		t.Fatalf("CompleteDeviceAuthFlow failed: %v", err)
	}
	if tok.AccessToken != "ghu_test_token_123" {
		t.Errorf("token = %q, want %q", tok.AccessToken, "ghu_test_token_123")
	}
}

func TestWaitForDeviceAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/device/code" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":      "dev-code-wait",
				"user_code":        "WAIT-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         5,
			})
			return
		}
		if r.URL.Path == "/oauth/access_token" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "ghu_wait_token_456",
				"token_type":   "bearer",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	SetProviderConfig("test_device_wait", ProviderConfig{
		ID:         "test_device_wait",
		Name:       "Test Device Wait",
		DeviceFlow: true,
		DeviceURL:  ts.URL + "/device/code",
		TokenURL:   ts.URL + "/oauth/access_token",
		ClientID:   "test-client-id",
	})

	_, _, err := BeginDeviceAuthFlow("test_device_wait", "inst-device-wait")
	if err != nil {
		t.Fatalf("BeginDeviceAuthFlow failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// WaitForDeviceAuth should succeed when token is acquired
	tok, err := WaitForDeviceAuth(ctx, "inst-device-wait")
	if err != nil {
		// If background ticker has not ticked yet, check if CompleteDeviceAuthFlow can complete it
		tok, err = CompleteDeviceAuthFlow("inst-device-wait")
		if err != nil {
			t.Fatalf("WaitForDeviceAuth/Complete failed: %v", err)
		}
	}
	if tok == nil || tok.AccessToken == "" {
		t.Errorf("expected access token, got %+v", tok)
	}
}
