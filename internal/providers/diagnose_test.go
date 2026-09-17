package providers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmnote/aigauge/internal/auth"
)

// fakeRunner replays a recorded command result and remembers what it was asked
// to run, so a test can assert both the resulting state and - just as important
// for the credential-first path - that no command was executed at all.
type fakeRunner struct {
	result commandResult
	err    error
	calls  [][]string
}

func (r *fakeRunner) run(_ context.Context, name string, args ...string) (commandResult, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.result, r.err
}

func foundPath(path string) pathLookup {
	return func(string) (string, error) { return path, nil }
}

func missingPath() pathLookup {
	return func(string) (string, error) { return "", exec.ErrNotFound }
}

// cliDeps wires a diagnosis to a fake CLI, the seams the agy-CLI-backed
// Antigravity path uses. Unlike testTokenDeps, no getToken/canImport is set:
// Antigravity's diagnosis never consults the token store.
func cliDeps(runner commandRunner, lookPath pathLookup) providerDeps {
	return providerDeps{
		runner:     runner,
		lookPath:   lookPath,
		homeDir:    func() (string, error) { return filepath.FromSlash("/home/tester"), nil },
		pathExists: func(string) bool { return false },
		readFile:   func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
}

// scriptedRunner answers differently per command, keyed by the first argument
// ("--version", "-p" for the /usage prompt, "models"). The Antigravity path
// chains up to three commands, and the interesting cases are exactly the ones
// where they disagree - /usage fails but models succeeds, say.
type scriptedRunner struct {
	results map[string]commandResult
	errs    map[string]error
	calls   [][]string
}

func (r *scriptedRunner) run(_ context.Context, name string, args ...string) (commandResult, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	key := ""
	if len(args) > 0 {
		key = args[0]
	}
	return r.results[key], r.errs[key]
}

func (r *scriptedRunner) ran(key string) bool {
	for _, call := range r.calls {
		for _, arg := range call {
			if arg == key {
				return true
			}
		}
	}
	return false
}

func antigravityRunner(results map[string]commandResult) *scriptedRunner {
	if _, ok := results["--version"]; !ok {
		results["--version"] = commandResult{Stdout: "1.1.28"}
	}
	return &scriptedRunner{results: results, errs: map[string]error{}}
}

func testTokenDeps(tokens map[string]*auth.Token, importable map[string]bool) providerDeps {
	return providerDeps{
		getToken: func(provider string) (*auth.Token, error) {
			if tokens != nil {
				return tokens[provider], nil
			}
			return nil, nil
		},
		canImport: func(provider string) bool {
			if importable != nil {
				return importable[provider]
			}
			return false
		},
	}
}

func TestDiagnoseClaudeWithToken(t *testing.T) {
	deps := testTokenDeps(map[string]*auth.Token{
		"claude": {AccessToken: "test-claude-token", Extra: auth.Extra{Plan: "pro"}},
	}, nil)

	// When active, returns ok=true with credentials
	diagActive, creds, ok := diagnoseClaude(context.Background(), deps, "claude", true)
	if !ok {
		t.Fatalf("diagnoseClaude(active) ok = false (%#v), want true", diagActive)
	}
	if creds.ClaudeAiOauth.AccessToken != "test-claude-token" {
		t.Errorf("AccessToken = %q, want %q", creds.ClaudeAiOauth.AccessToken, "test-claude-token")
	}
	if creds.ClaudeAiOauth.SubscriptionType != "pro" {
		t.Errorf("SubscriptionType = %q, want %q", creds.ClaudeAiOauth.SubscriptionType, "pro")
	}

	// When inactive, stops at StatusAuthCheckRequired
	diagInactive, _, ok := diagnoseClaude(context.Background(), deps, "claude", false)
	if ok {
		t.Error("diagnoseClaude(inactive) ok = true, want false")
	}
	if diagInactive.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagInactive.Status, StatusAuthCheckRequired)
	}
	if !strings.Contains(diagInactive.Message, "Credentials found") {
		t.Errorf("Message = %q, want 'Credentials found'", diagInactive.Message)
	}
}

func TestDiagnoseClaudeWithoutToken(t *testing.T) {
	// Without token, but importable
	depsImportable := testTokenDeps(nil, map[string]bool{"claude": true})
	diag, _, ok := diagnoseClaude(context.Background(), depsImportable, "claude", true)
	if ok {
		t.Error("diagnoseClaude without token ok = true, want false")
	}
	if diag.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diag.Status, StatusLoginRequired)
	}
	if !diag.CanImport {
		t.Error("CanImport = false, want true when legacy credentials exist")
	}

	// Without token and not importable
	depsNotImportable := testTokenDeps(nil, map[string]bool{"claude": false})
	diag2, _, _ := diagnoseClaude(context.Background(), depsNotImportable, "claude", true)
	if diag2.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diag2.Status, StatusLoginRequired)
	}
	if diag2.CanImport {
		t.Error("CanImport = true, want false when legacy credentials do not exist")
	}
}

func TestDiagnoseCodexWithToken(t *testing.T) {
	deps := testTokenDeps(map[string]*auth.Token{
		"codex": {AccessToken: "test-codex-token", Extra: auth.Extra{Plan: "team"}},
	}, nil)

	// When active, returns ok=true with credentials
	diagActive, creds, ok := diagnoseCodex(context.Background(), deps, "codex", true)
	if !ok {
		t.Fatalf("diagnoseCodex(active) ok = false (%#v), want true", diagActive)
	}
	if creds.Tokens.AccessToken != "test-codex-token" {
		t.Errorf("AccessToken = %q, want %q", creds.Tokens.AccessToken, "test-codex-token")
	}

	// When inactive, stops at StatusAuthCheckRequired
	diagInactive, _, ok := diagnoseCodex(context.Background(), deps, "codex", false)
	if ok {
		t.Error("diagnoseCodex(inactive) ok = true, want false")
	}
	if diagInactive.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagInactive.Status, StatusAuthCheckRequired)
	}
	if !strings.Contains(diagInactive.Message, "Credentials found") {
		t.Errorf("Message = %q, want 'Credentials found'", diagInactive.Message)
	}
}

func TestDiagnoseCodexWithoutToken(t *testing.T) {
	// Without token, but importable
	depsImportable := testTokenDeps(nil, map[string]bool{"codex": true})
	diag, _, ok := diagnoseCodex(context.Background(), depsImportable, "codex", true)
	if ok {
		t.Error("diagnoseCodex without token ok = true, want false")
	}
	if diag.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diag.Status, StatusLoginRequired)
	}
	if !diag.CanImport {
		t.Error("CanImport = false, want true when legacy credentials exist")
	}

	// Without token and not importable
	depsNotImportable := testTokenDeps(nil, map[string]bool{"codex": false})
	diag2, _, _ := diagnoseCodex(context.Background(), depsNotImportable, "codex", true)
	if diag2.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diag2.Status, StatusLoginRequired)
	}
	if diag2.CanImport {
		t.Error("CanImport = true, want false when legacy credentials do not exist")
	}
}

func TestDiagnoseAntigravityStopsBeforeAnyNetworkCommandWhenInactive(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: "1.1.28"}}
	diagnosis, ok := diagnoseAntigravity(context.Background(), runner, "agy", false)
	if ok {
		t.Error("diagnoseAntigravity() ok = true, want false so /usage is never run for an inactive provider")
	}
	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if len(runner.calls) != 1 || runner.calls[0][1] != "--version" {
		t.Errorf("calls = %v, want exactly one --version call", runner.calls)
	}
}

func TestDiagnoseAntigravityReportsUnsupportedCLI(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "unknown flag: --version", ExitCode: 2}}
	diagnosis, ok := diagnoseAntigravity(context.Background(), runner, "agy", true)
	if ok {
		t.Error("diagnoseAntigravity() ok = true, want false for an unsupported CLI")
	}
	if diagnosis.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusUnsupportedCLI)
	}
	if !strings.Contains(diagnosis.Message, antigravityInstallGuideURL) {
		t.Errorf("Message = %q, want it to contain %q", diagnosis.Message, antigravityInstallGuideURL)
	}
}

func TestDiagnoseAntigravityProceedsWhenActive(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: "1.1.28"}}
	if _, ok := diagnoseAntigravity(context.Background(), runner, "agy", true); !ok {
		t.Error("diagnoseAntigravity() ok = false, want true so the caller runs /usage")
	}
}

func TestFindAgyReportsNotInstalledWithGuideLink(t *testing.T) {
	deps := cliDeps(&fakeRunner{}, missingPath())
	_, diagnosis, ok := findAgy(deps)
	if ok {
		t.Error("findAgy() ok = true, want false when agy is not found")
	}
	if diagnosis.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusNotInstalled)
	}
	if !strings.Contains(diagnosis.Message, antigravityInstallGuideURL) {
		t.Errorf("Message = %q, want it to contain %q", diagnosis.Message, antigravityInstallGuideURL)
	}
}

func TestDiagnoseAntigravityLocalNeverRunsANetworkCommand(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	diagnosis := diagnoseAntigravityLocal(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity")

	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if runner.ran("-p") || runner.ran("models") {
		t.Errorf("calls = %v, want only --version for a local-only diagnosis", runner.calls)
	}
}

func TestGetAntigravityUsageReportsNotInstalledWithoutRawPathError(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, missingPath()), "antigravity", true)

	if usage.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", usage.Status, StatusNotInstalled)
	}
	if !strings.Contains(usage.Message, "agy") {
		t.Errorf("Message = %q, want it to name the agy CLI", usage.Message)
	}
}

func TestGetAntigravityUsageStopsBeforeUsageWhenInactive(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", false)

	if usage.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusAuthCheckRequired)
	}
	if runner.ran("-p") {
		t.Errorf("calls = %v, want /usage never run for an inactive check", runner.calls)
	}
}

func TestGetAntigravityUsageConnectsOnValidUsage(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stdout: string(readFixture(t, "antigravity-usage.json"))},
	})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if usage.Status != StatusConnected {
		t.Fatalf("Status = %q (%q), want %q", usage.Status, usage.Message, StatusConnected)
	}
	if len(usage.Groups) == 0 {
		t.Fatal("Groups is empty, want at least one group from the fixture")
	}
}

func TestGetAntigravityUsageSeparatesNoDataFromUnreadableResponse(t *testing.T) {
	empty := antigravityRunner(map[string]commandResult{"-p": {Stdout: `{"command":{"data":{"groups":[]}}}`}})
	usage := getAntigravityUsage(context.Background(), cliDeps(empty, foundPath("agy")), "antigravity", true)
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonNoUsageData {
		t.Errorf("empty groups: (Status, Reason) = (%q, %q), want (%q, %q)",
			usage.Status, usage.Reason, StatusUsageUnavailable, ReasonNoUsageData)
	}

	garbled := antigravityRunner(map[string]commandResult{"-p": {Stdout: "not json at all"}})
	usage = getAntigravityUsage(context.Background(), cliDeps(garbled, foundPath("agy")), "antigravity", true)
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonUnsupportedResponse {
		t.Errorf("garbled output: (Status, Reason) = (%q, %q), want (%q, %q)",
			usage.Status, usage.Reason, StatusUsageUnavailable, ReasonUnsupportedResponse)
	}
}

func TestGetAntigravityUsageReportsLoginRequiredOnAuthMarker(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stderr: "Error: not logged in. Run agy to sign in.", ExitCode: 1},
	})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if usage.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusLoginRequired)
	}
	if !runner.ran("models") {
		t.Errorf("calls = %v, want models authentication check before usage", runner.calls)
	}
}

func TestGetAntigravityUsageReportsUnsupportedCLIOnRejectedCommand(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stderr: `Error: unexpected argument "--output-format".`, ExitCode: 2},
	})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if usage.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", usage.Status, StatusUnsupportedCLI)
	}
}

func TestGetAntigravityUsageUsesModelsToDisambiguateAnUnreadableFailure(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p":     {Stderr: "Error: something unexpected happened", ExitCode: 1},
		"models": {Stdout: "gemini-3-pro\ngpt-5.2\n", Stderr: "Fetching available models..."},
	})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if !runner.ran("models") {
		t.Fatalf("ran %v, want the secondary diagnostic for an ambiguous failure", runner.calls)
	}
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonUnsupportedResponse {
		t.Errorf("(Status, Reason) = (%q, %q), want (%q, %q)", usage.Status, usage.Reason, StatusUsageUnavailable, ReasonUnsupportedResponse)
	}
}

func TestGetAntigravityUsageNeverGuessesSignOutFromAnUnknownFailure(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p":     {Stderr: "Error: something unexpected happened", ExitCode: 1},
		"models": {Stderr: "Error: something unexpected happened", ExitCode: 1},
	})
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if usage.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q - neither command said anything about authentication", usage.Status, StatusTemporaryError)
	}
}

func TestGetAntigravityUsageReportsTemporaryErrorOnUsageTimeout(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	runner.errs["-p"] = context.DeadlineExceeded
	usage := getAntigravityUsage(context.Background(), cliDeps(runner, foundPath("agy")), "antigravity", true)

	if usage.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q", usage.Status, StatusTemporaryError)
	}
}

func TestGetClaudeUsageWithoutToken(t *testing.T) {
	depsImportable := testTokenDeps(nil, map[string]bool{"claude": true})
	usage := getClaudeUsage(context.Background(), depsImportable, "claude", true)
	if usage.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusLoginRequired)
	}
	if !usage.CanImport {
		t.Error("CanImport = false, want true")
	}
}

func TestGetCodexUsageWithoutToken(t *testing.T) {
	depsImportable := testTokenDeps(nil, map[string]bool{"codex": true})
	usage := getCodexUsage(context.Background(), depsImportable, "codex", true)
	if usage.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusLoginRequired)
	}
	if !usage.CanImport {
		t.Error("CanImport = false, want true")
	}
}

func TestLocalDiagnosisNeverReportsConnected(t *testing.T) {
	deps := testTokenDeps(map[string]*auth.Token{
		"claude": {AccessToken: "token-claude", Extra: auth.Extra{Plan: "pro"}},
		"codex":  {AccessToken: "token-codex", Extra: auth.Extra{Plan: "team"}},
	}, nil)

	claude, _, _ := diagnoseClaude(context.Background(), deps, "claude", false)
	codex, _, _ := diagnoseCodex(context.Background(), deps, "codex", false)
	antigravity := diagnoseAntigravityLocal(context.Background(),
		cliDeps(antigravityRunner(map[string]commandResult{}), foundPath("agy")), "antigravity")

	for name, diagnosis := range map[string]Diagnosis{"claude": claude, "codex": codex, "antigravity": antigravity} {
		if diagnosis.Status == StatusConnected {
			t.Errorf("%s: Status = %q, want any state but connected from a local-only diagnosis", name, diagnosis.Status)
		}
		if diagnosis.Status != StatusAuthCheckRequired {
			t.Errorf("%s: Status = %q, want %q", name, diagnosis.Status, StatusAuthCheckRequired)
		}
	}
}

func TestUsageFailureDiagnosisMapsUnauthorizedToSignIn(t *testing.T) {
	for _, code := range []int{401, 403} {
		err := &httpStatusError{StatusCode: code, message: "Claude usage request failed"}
		if diagnosis := usageFailureDiagnosis("Claude", err); diagnosis.Status != StatusLoginRequired {
			t.Errorf("HTTP %d: Status = %q, want %q", code, diagnosis.Status, StatusLoginRequired)
		}
	}
}

func TestUsageFailureDiagnosisMapsOtherFailuresToTemporaryError(t *testing.T) {
	serverErr := &httpStatusError{StatusCode: 503, message: "Claude usage request failed (HTTP 503)"}
	if diagnosis := usageFailureDiagnosis("Claude", serverErr); diagnosis.Status != StatusTemporaryError {
		t.Errorf("HTTP 503: Status = %q, want %q", diagnosis.Status, StatusTemporaryError)
	}
	transportErr := errors.New("dial tcp: lookup api.anthropic.com: no such host")
	if diagnosis := usageFailureDiagnosis("Claude", transportErr); diagnosis.Status != StatusTemporaryError {
		t.Errorf("transport failure: Status = %q, want %q", diagnosis.Status, StatusTemporaryError)
	}
}

func TestUsageUnreadableDiagnosisKeepsTheReasonsApart(t *testing.T) {
	noData := usageUnreadableDiagnosis("Claude", ReasonNoUsageData, errNoUsageWindows)
	badShape := usageUnreadableDiagnosis("Claude", ReasonUnsupportedResponse, errors.New("unexpected field"))
	if noData.Status != StatusUsageUnavailable || badShape.Status != StatusUsageUnavailable {
		t.Fatalf("statuses = (%q, %q), want both %q", noData.Status, badShape.Status, StatusUsageUnavailable)
	}
	if noData.Reason == badShape.Reason {
		t.Errorf("both reasons = %q, want them distinguished so the card can offer different guidance", noData.Reason)
	}
	if noData.Message == badShape.Message {
		t.Errorf("both messages = %q, want different guidance per reason", noData.Message)
	}
}

func TestNeedsUserActionCoversTheExpectedSetupStates(t *testing.T) {
	expected := map[Status]bool{
		StatusAuthCheckRequired: true,
		StatusLoginRequired:     true,
		StatusAwaitingCode:      true,
		StatusNotInstalled:      true,
		StatusConnected:         false,
		StatusUsageUnavailable:  false,
		StatusTemporaryError:    false,
		StatusUnsupportedCLI:    false,
	}
	for status, want := range expected {
		if got := status.NeedsUserAction(); got != want {
			t.Errorf("%q.NeedsUserAction() = %v, want %v", status, got, want)
		}
	}
}

func TestTechnicalDetailsRedactsCredentials(t *testing.T) {
	raw := "signed in as someone@example.com org 7f3a1c02-9b44-4e21-8d55-0c1e2a6b9f80 " +
		"token sk-abcdef0123456789 header Bearer eyJhbGciOiJIUzI1NiJ9 " +
		"opaque AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHH1234"
	details := technicalDetails(raw)
	for _, secret := range []string{
		"someone@example.com",
		"7f3a1c02-9b44-4e21-8d55-0c1e2a6b9f80",
		"sk-abcdef0123456789",
		"eyJhbGciOiJIUzI1NiJ9",
		"AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHH1234",
	} {
		if strings.Contains(details, secret) {
			t.Errorf("technicalDetails() leaked %q in %q", secret, details)
		}
	}
}

func TestTechnicalDetailsCollapsesAndTruncates(t *testing.T) {
	if got := technicalDetails("   \n\t  "); got != "" {
		t.Errorf("technicalDetails(whitespace) = %q, want an empty string", got)
	}
	if got := technicalDetails("first line\n\nsecond   line"); got != "first line second line" {
		t.Errorf("technicalDetails() = %q, want collapsed whitespace", got)
	}
	long := technicalDetails(strings.Repeat("가 ", 2*maxDetailLength))
	if len(long) > maxDetailLength+3 {
		t.Errorf("len(technicalDetails()) = %d, want at most %d", len(long), maxDetailLength+3)
	}
	if !strings.HasSuffix(long, "...") {
		t.Errorf("technicalDetails() = %q, want a truncation marker", long)
	}
}

func TestAuthorizedAccessTokenPrefersARefreshedToken(t *testing.T) {
	var gotCfgType, gotTokenKey string
	deps := providerDeps{
		getValidAccessToken: func(ctx context.Context, cfgType, tokenKey string) (string, error) {
			gotCfgType, gotTokenKey = cfgType, tokenKey
			return "refreshed-token", nil
		},
	}
	got, err := authorizedAccessToken(context.Background(), deps, "codex", "instance-1", "stale-token")
	if err != nil {
		t.Fatalf("authorizedAccessToken() error = %v", err)
	}
	if got != "refreshed-token" {
		t.Errorf("authorizedAccessToken() = %q, want the refreshed token", got)
	}
	if gotCfgType != "codex" || gotTokenKey != "instance-1" {
		t.Errorf("getValidAccessToken called with (%q, %q), want (codex, instance-1)", gotCfgType, gotTokenKey)
	}
}

func TestAuthorizedAccessTokenReturnsRefreshFailure(t *testing.T) {
	deps := providerDeps{
		getValidAccessToken: func(ctx context.Context, cfgType, tokenKey string) (string, error) {
			return "", errors.New("refresh failed")
		},
	}
	got, err := authorizedAccessToken(context.Background(), deps, "codex", "instance-1", "stale-token")
	if err == nil || got != "" {
		t.Errorf("authorizedAccessToken() = (%q, %v), want empty token and refresh error", got, err)
	}
}

func TestAuthorizedAccessTokenFallsBackWhenNotWired(t *testing.T) {
	// The diagnose-only tests above build a providerDeps with no
	// getValidAccessToken at all; a usage fetch given that same deps must
	// degrade to the credential's own token rather than panic.
	got, err := authorizedAccessToken(context.Background(), providerDeps{}, "codex", "instance-1", "stale-token")
	if err != nil {
		t.Fatalf("authorizedAccessToken() error = %v", err)
	}
	if got != "stale-token" {
		t.Errorf("authorizedAccessToken() = %q, want the fallback token when unwired", got)
	}
}
