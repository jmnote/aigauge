package providers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

// testDeps wires a diagnosis to a fake CLI and a fake home directory. files is
// keyed by slash-separated suffix (".claude/.credentials.json"), which keeps the
// tests readable and portable across the separator the code joins with.
func testDeps(runner commandRunner, lookPath pathLookup, files map[string]string) providerDeps {
	return providerDeps{
		runner:     runner,
		lookPath:   lookPath,
		homeDir:    func() (string, error) { return filepath.FromSlash("/home/tester"), nil },
		pathExists: func(string) bool { return false },
		readFile: func(path string) ([]byte, error) {
			for suffix, content := range files {
				if strings.HasSuffix(filepath.ToSlash(path), suffix) {
					return []byte(content), nil
				}
			}
			return nil, os.ErrNotExist
		},
	}
}

const claudeCredentialFile = `{"claudeAiOauth":{"accessToken":"test-token","subscriptionType":"pro"}}`
const codexCredentialFile = `{"tokens":{"access_token":"test-token"}}`

func TestDiagnoseClaudeUsesCredentialsWithoutConsultingTheCLI(t *testing.T) {
	runner := &fakeRunner{}
	deps := testDeps(runner, missingPath(), map[string]string{".claude/.credentials.json": claudeCredentialFile})

	diagnosis, credentials, ok := diagnoseClaude(context.Background(), deps, true)
	if !ok {
		t.Fatalf("diagnoseClaude() ok = false (%#v), want true - a readable credential is all the usage request needs", diagnosis)
	}
	if credentials.ClaudeAiOauth.AccessToken != "test-token" {
		t.Errorf("AccessToken = %q, want the token from the credential file", credentials.ClaudeAiOauth.AccessToken)
	}
	// The lookup above reports the CLI as missing on purpose: a user whose
	// `claude` binary is not on the PATH this app inherited must still get quota.
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no CLI command when a credential was found", runner.calls)
	}
}

func TestDiagnoseClaudeStopsAtAuthCheckWhenProviderIsInactive(t *testing.T) {
	runner := &fakeRunner{}
	deps := testDeps(runner, foundPath("claude"), map[string]string{".claude/.credentials.json": claudeCredentialFile})

	diagnosis, _, ok := diagnoseClaude(context.Background(), deps, false)
	if ok {
		t.Error("diagnoseClaude() ok = true, want false so an inactive provider never reaches the usage request")
	}
	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if !strings.Contains(diagnosis.Message, "Credentials found") {
		t.Errorf("Message = %q, want the credential-based explanation", diagnosis.Message)
	}
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no command for an inactive provider with a credential", runner.calls)
	}
}

func TestDiagnoseClaudeFallsBackToTheCLIWhenCredentialsAreUnreadable(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: string(readFixture(t, "claude-auth-status-signed-out.json"))}}
	deps := testDeps(runner, foundPath("claude"), map[string]string{".claude/.credentials.json": "{ not json"})

	diagnosis, _, _ := diagnoseClaude(context.Background(), deps, true)
	if diagnosis.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusLoginRequired)
	}
	want := []string{"claude", "auth", "status", "--json"}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != strings.Join(want, " ") {
		t.Errorf("ran %v, want a single %v", runner.calls, want)
	}
}

func TestDiagnoseClaudeReportsNotInstalledOnlyWhenNothingIsFound(t *testing.T) {
	runner := &fakeRunner{}
	deps := testDeps(runner, missingPath(), nil)

	diagnosis, _, ok := diagnoseClaude(context.Background(), deps, true)
	if ok {
		t.Error("diagnoseClaude() ok = true, want false with neither credential nor CLI")
	}
	if diagnosis.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusNotInstalled)
	}
	if !strings.Contains(diagnosis.Details, "CLI Not Found") {
		t.Errorf("Details = %q, want Details to mention CLI Not Found", diagnosis.Details)
	}
	if strings.Contains(diagnosis.Details, "credentials") {
		t.Errorf("Details = %q, want Details not to mention credentials file", diagnosis.Details)
	}
	wantLink := "https://code.claude.com/docs/ko/quickstart#step-1-install-claude-code"
	if !strings.Contains(diagnosis.Message, wantLink) {
		t.Errorf("Message = %q, want it to contain %q", diagnosis.Message, wantLink)
	}
}

func TestDiagnoseClaudeReportsSignedInLocallyWhenInactiveWithoutCredentials(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: string(readFixture(t, "claude-auth-status-signed-in.json"))}}
	deps := testDeps(runner, foundPath("claude"), nil)

	diagnosis, _, _ := diagnoseClaude(context.Background(), deps, false)
	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if !strings.Contains(diagnosis.Message, "Logged in locally") {
		t.Errorf("Message = %q, want the CLI-based explanation", diagnosis.Message)
	}
}

func TestDiagnoseClaudeReportsUnsupportedCredentialSource(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: string(readFixture(t, "claude-auth-status-signed-in.json"))}}
	deps := testDeps(runner, foundPath("claude"), nil)

	diagnosis, _, ok := diagnoseClaude(context.Background(), deps, true)
	if ok {
		t.Error("diagnoseClaude() ok = true, want false - there is no token to make the request with")
	}
	if diagnosis.Status != StatusUsageUnavailable || diagnosis.Reason != ReasonUnsupportedCredentialSource {
		t.Errorf("(Status, Reason) = (%q, %q), want (%q, %q)",
			diagnosis.Status, diagnosis.Reason, StatusUsageUnavailable, ReasonUnsupportedCredentialSource)
	}
}

func TestDiagnoseClaudeTrustsReadableJSONOverExitCode(t *testing.T) {
	// The signed-out exit code is still an open item in the release gate, so a
	// parseable answer decides the state even when the process exited non-zero.
	runner := &fakeRunner{result: commandResult{
		Stdout:   string(readFixture(t, "claude-auth-status-signed-out.json")),
		ExitCode: 1,
	}}
	diagnosis, _, _ := diagnoseClaude(context.Background(), testDeps(runner, foundPath("claude"), nil), true)
	if diagnosis.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q even though the command exited non-zero", diagnosis.Status, StatusLoginRequired)
	}
}

func TestDiagnoseClaudeReportsUnsupportedCLIForRejectedCommand(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "unknown command: auth", ExitCode: 2}}
	diagnosis, _, _ := diagnoseClaude(context.Background(), testDeps(runner, foundPath("claude"), nil), true)
	if diagnosis.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusUnsupportedCLI)
	}
}

func TestDiagnoseClaudeDoesNotInferSignOutFromUnknownFailure(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "panic: something went wrong", ExitCode: 3}}
	diagnosis, _, _ := diagnoseClaude(context.Background(), testDeps(runner, foundPath("claude"), nil), true)
	if diagnosis.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q - an unrecognized failure must not be reported as signed out", diagnosis.Status, StatusTemporaryError)
	}
}

func TestDiagnoseClaudeReportsTemporaryErrorOnTimeout(t *testing.T) {
	runner := &fakeRunner{err: context.DeadlineExceeded}
	diagnosis, _, ok := diagnoseClaude(context.Background(), testDeps(runner, foundPath("claude"), nil), true)
	if ok {
		t.Error("diagnoseClaude() ok = true, want false on a timeout")
	}
	if diagnosis.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusTemporaryError)
	}
}

func TestDiagnoseCodexUsesCredentialsWithoutConsultingTheCLI(t *testing.T) {
	runner := &fakeRunner{}
	deps := testDeps(runner, missingPath(), map[string]string{".codex/auth.json": codexCredentialFile})

	diagnosis, credentials, ok := diagnoseCodex(context.Background(), deps, true)
	if !ok {
		t.Fatalf("diagnoseCodex() ok = false (%#v), want true", diagnosis)
	}
	if credentials.Tokens.AccessToken != "test-token" {
		t.Errorf("AccessToken = %q, want the token from the credential file", credentials.Tokens.AccessToken)
	}
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no CLI command when a credential was found", runner.calls)
	}
}

func TestDiagnoseCodexStopsAtAuthCheckWhenProviderIsInactive(t *testing.T) {
	runner := &fakeRunner{}
	deps := testDeps(runner, foundPath("codex"), map[string]string{".codex/auth.json": codexCredentialFile})

	diagnosis, _, ok := diagnoseCodex(context.Background(), deps, false)
	if ok {
		t.Error("diagnoseCodex() ok = true, want false so an inactive provider never reaches the usage request")
	}
	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
}

func TestDiagnoseCodexReportsLoginRequiredOnNonZeroExit(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "Not logged in", ExitCode: 1}}
	diagnosis, _, _ := diagnoseCodex(context.Background(), testDeps(runner, foundPath("codex"), nil), true)
	if diagnosis.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusLoginRequired)
	}
}

func TestDiagnoseCodexReportsUnsupportedCLIForRejectedCommand(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "error: unknown subcommand 'status'", ExitCode: 2}}
	diagnosis, _, _ := diagnoseCodex(context.Background(), testDeps(runner, foundPath("codex"), nil), true)
	if diagnosis.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusUnsupportedCLI)
	}
}

func TestDiagnoseCodexReportsUnsupportedCredentialSource(t *testing.T) {
	// Verified behavior of codex-cli 0.146.0 when signed in: exit 0, empty
	// stdout, the message on stderr. With no readable credential file, that
	// combination means the session is stored somewhere this app cannot use.
	runner := &fakeRunner{result: commandResult{Stderr: "Logged in", ExitCode: 0}}
	diagnosis, _, ok := diagnoseCodex(context.Background(), testDeps(runner, foundPath("codex"), nil), true)
	if ok {
		t.Error("diagnoseCodex() ok = true, want false - there is no token to make the request with")
	}
	if diagnosis.Status != StatusUsageUnavailable || diagnosis.Reason != ReasonUnsupportedCredentialSource {
		t.Errorf("(Status, Reason) = (%q, %q), want (%q, %q)",
			diagnosis.Status, diagnosis.Reason, StatusUsageUnavailable, ReasonUnsupportedCredentialSource)
	}
}

func TestDiagnoseCodexReportsNotInstalledOnlyWhenNothingIsFound(t *testing.T) {
	diagnosis, _, _ := diagnoseCodex(context.Background(), testDeps(&fakeRunner{}, missingPath(), nil), true)
	if diagnosis.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusNotInstalled)
	}
	if !strings.Contains(diagnosis.Details, "CLI Not Found") {
		t.Errorf("Details = %q, want Details to mention CLI Not Found", diagnosis.Details)
	}
	if strings.Contains(diagnosis.Details, "credentials") {
		t.Errorf("Details = %q, want Details not to mention credentials file", diagnosis.Details)
	}
	wantLink := "https://learn.chatgpt.com/docs/codex/cli#getting-started"
	if !strings.Contains(diagnosis.Message, wantLink) {
		t.Errorf("Message = %q, want it to contain %q", diagnosis.Message, wantLink)
	}
}

// diagnoseAntigravity only ever backs the offline onboarding screen now -
// see getAntigravityUsage for the active path, which runs `/usage` directly
// instead of calling this first.
func TestDiagnoseAntigravityReportsAuthCheckRequiredOnSupportedVersion(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stdout: "1.1.28"}}
	diagnosis := diagnoseAntigravity(context.Background(), runner, "agy")
	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if len(runner.calls) != 1 || runner.calls[0][1] != "--version" {
		t.Errorf("ran %v, want only the local `agy --version` check", runner.calls)
	}
}

func TestDiagnoseAntigravityReportsUnsupportedCLI(t *testing.T) {
	runner := &fakeRunner{result: commandResult{Stderr: "unknown flag: --version", ExitCode: 2}}
	diagnosis := diagnoseAntigravity(context.Background(), runner, "agy")
	if diagnosis.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusUnsupportedCLI)
	}
}

func TestFindAgyReportsNotInstalledWithGuideLink(t *testing.T) {
	deps := testDeps(&fakeRunner{}, missingPath(), nil)
	_, diagnosis, ok := findAgy(deps)
	if ok {
		t.Error("findAgy() ok = true, want false when agy is not installed")
	}
	if diagnosis.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusNotInstalled)
	}
	wantLink := "https://antigravity.google/docs/cli/install"
	if !strings.Contains(diagnosis.Message, wantLink) {
		t.Errorf("Message = %q, want it to contain %q", diagnosis.Message, wantLink)
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
		StatusNotInstalled:      true,
		StatusAuthCheckRequired: true,
		StatusLoginRequired:     true,
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

func TestLimitedBufferStopsAtTheLimitWithoutShortWrites(t *testing.T) {
	buffer := &limitedBuffer{limit: 8}
	written, err := buffer.Write([]byte("0123456789"))
	if err != nil || written != 10 {
		t.Errorf("Write() = (%d, %v), want (10, nil) so an oversized write never kills the child process", written, err)
	}
	if got := buffer.String(); got != "01234567" {
		t.Errorf("String() = %q, want the first 8 bytes only", got)
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
	key := ""
	if len(args) > 0 {
		key = args[0]
	}
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.results[key], r.errs[key]
}

func (r *scriptedRunner) ran(key string) bool {
	for _, call := range r.calls {
		if len(call) > 1 && call[1] == key {
			return true
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

func TestGetAntigravityUsageReportsNotInstalledWithoutRawPathError(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, missingPath(), nil), true)

	if usage.Status != StatusNotInstalled {
		t.Errorf("Status = %q, want %q", usage.Status, StatusNotInstalled)
	}
	// Message provides clean, actionable guidance that names the command to install.
	if strings.Contains(usage.Message, "PATH") {
		t.Errorf("Message = %q, want guidance rather than the raw lookup error", usage.Message)
	}
	if !strings.Contains(usage.Message, "agy") {
		t.Errorf("Message = %q, want guidance to name the agy CLI", usage.Message)
	}
	// Details provides concrete lookup locations so the user can see what was searched.
	if usage.Details == "" || !strings.Contains(usage.Details, "CLI Not Found") {
		t.Errorf("Details = %q, want Details to mention CLI Not Found", usage.Details)
	}
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no command when the executable was never resolved", runner.calls)
	}
}

func TestGetAntigravityUsageStopsBeforeUsageWhenInactive(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), false)

	if usage.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusAuthCheckRequired)
	}
	if runner.ran("-p") {
		t.Errorf("ran %v, want no /usage request for an inactive provider", runner.calls)
	}
}

func TestGetAntigravityUsageConnectsOnValidUsage(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stdout: string(readFixture(t, "antigravity-usage.json"))},
	})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if usage.Status != StatusConnected {
		t.Fatalf("Status = %q (%q), want %q", usage.Status, usage.Message, StatusConnected)
	}
	if len(usage.Groups) == 0 {
		t.Error("Groups is empty, want the parsed usage groups")
	}
	if usage.Error != "" {
		t.Errorf("Error = %q, want empty on success", usage.Error)
	}
	if runner.ran("models") {
		t.Errorf("ran %v, want no secondary diagnostic after a clean /usage", runner.calls)
	}
	// Checking the connection IS running /usage - it must not also run a
	// separate `agy --version` first, since that would ask the same
	// executable/version question /usage already answers a second time.
	if runner.ran("--version") || len(runner.calls) != 1 {
		t.Errorf("ran %v, want exactly one command (`/usage`) for an active check", runner.calls)
	}
}

func TestGetAntigravityUsageSeparatesNoDataFromUnreadableResponse(t *testing.T) {
	empty := antigravityRunner(map[string]commandResult{"-p": {Stdout: `{"command":{"data":{"groups":[]}}}`}})
	usage := getAntigravityUsage(context.Background(), testDeps(empty, foundPath("agy"), nil), true)
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonNoUsageData {
		t.Errorf("empty groups: (Status, Reason) = (%q, %q), want (%q, %q)",
			usage.Status, usage.Reason, StatusUsageUnavailable, ReasonNoUsageData)
	}

	garbled := antigravityRunner(map[string]commandResult{"-p": {Stdout: "not json at all"}})
	usage = getAntigravityUsage(context.Background(), testDeps(garbled, foundPath("agy"), nil), true)
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonUnsupportedResponse {
		t.Errorf("garbled output: (Status, Reason) = (%q, %q), want (%q, %q)",
			usage.Status, usage.Reason, StatusUsageUnavailable, ReasonUnsupportedResponse)
	}
}

func TestGetAntigravityUsageReportsLoginRequiredOnAuthMarker(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stderr: "Error: not logged in. Run agy to sign in.", ExitCode: 1},
	})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if usage.Status != StatusLoginRequired {
		t.Errorf("Status = %q, want %q", usage.Status, StatusLoginRequired)
	}
	if runner.ran("models") {
		t.Errorf("ran %v, want no secondary diagnostic once the failure is already clear", runner.calls)
	}
}

func TestGetAntigravityUsageReportsUnsupportedCLIOnRejectedCommand(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p": {Stderr: `Error: unexpected argument "--output-format".`, ExitCode: 2},
	})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if usage.Status != StatusUnsupportedCLI {
		t.Errorf("Status = %q, want %q", usage.Status, StatusUnsupportedCLI)
	}
}

func TestGetAntigravityUsageUsesModelsToDisambiguateAnUnreadableFailure(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p":     {Stderr: "Error: something unexpected happened", ExitCode: 1},
		"models": {Stdout: "gemini-3-pro\ngpt-5.2\n", Stderr: "Fetching available models..."},
	})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if !runner.ran("models") {
		t.Fatalf("ran %v, want the secondary diagnostic for an ambiguous failure", runner.calls)
	}
	// models answering normally proves the session is fine, so the failure
	// belongs to the usage response rather than to authentication.
	if usage.Status != StatusUsageUnavailable || usage.Reason != ReasonUnsupportedResponse {
		t.Errorf("(Status, Reason) = (%q, %q), want (%q, %q)",
			usage.Status, usage.Reason, StatusUsageUnavailable, ReasonUnsupportedResponse)
	}
}

func TestGetAntigravityUsageNeverGuessesSignOutFromAnUnknownFailure(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{
		"-p":     {Stderr: "Error: something unexpected happened", ExitCode: 1},
		"models": {Stderr: "Error: something unexpected happened", ExitCode: 1},
	})
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if usage.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q - neither command said anything about authentication", usage.Status, StatusTemporaryError)
	}
}

func TestGetAntigravityUsageReportsTemporaryErrorOnUsageTimeout(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	runner.errs["-p"] = context.DeadlineExceeded
	usage := getAntigravityUsage(context.Background(), testDeps(runner, foundPath("agy"), nil), true)

	if usage.Status != StatusTemporaryError {
		t.Errorf("Status = %q, want %q", usage.Status, StatusTemporaryError)
	}
}

func TestDiagnoseAntigravityLocalNeverRunsANetworkCommand(t *testing.T) {
	runner := antigravityRunner(map[string]commandResult{})
	diagnosis := diagnoseAntigravityLocal(context.Background(), testDeps(runner, foundPath("agy"), nil))

	if diagnosis.Status != StatusAuthCheckRequired {
		t.Errorf("Status = %q, want %q", diagnosis.Status, StatusAuthCheckRequired)
	}
	if runner.ran("-p") || runner.ran("models") {
		t.Errorf("ran %v, want only the local version check on the first-run screen", runner.calls)
	}
}

func TestLocalDiagnosisNeverReportsConnected(t *testing.T) {
	// The first-run screen must not be able to claim a working connection: only
	// a real usage response can, and none of these paths make one.
	claude, _, _ := diagnoseClaude(context.Background(),
		testDeps(&fakeRunner{}, foundPath("claude"), map[string]string{".claude/.credentials.json": claudeCredentialFile}), false)
	codex, _, _ := diagnoseCodex(context.Background(),
		testDeps(&fakeRunner{}, foundPath("codex"), map[string]string{".codex/auth.json": codexCredentialFile}), false)
	antigravity := diagnoseAntigravityLocal(context.Background(),
		testDeps(antigravityRunner(map[string]commandResult{}), foundPath("agy"), nil))

	for name, diagnosis := range map[string]Diagnosis{"claude": claude, "codex": codex, "antigravity": antigravity} {
		if diagnosis.Status == StatusConnected {
			t.Errorf("%s: Status = %q, want any state but connected from a local-only diagnosis", name, diagnosis.Status)
		}
		if diagnosis.Status != StatusAuthCheckRequired {
			t.Errorf("%s: Status = %q, want %q", name, diagnosis.Status, StatusAuthCheckRequired)
		}
	}
}
