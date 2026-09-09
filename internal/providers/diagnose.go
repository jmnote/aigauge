package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// statusCommandTimeout bounds a local status command. These commands read
// on-disk credential state and answer immediately; anything slower is a hung
// process, not a slow answer, and the provider card should say "try again"
// rather than sit on a spinner.
const statusCommandTimeout = 10 * time.Second

// unsupportedCLIMarkers are the common ways provider CLIs reject a command or
// option they do not implement. Command-line parsers normally return a non-zero
// exit code for these errors, so recognize them before the generic login or
// temporary-error branches.
var unsupportedCLIMarkers = []string{
	"unknown flag",
	"unknown command",
	"unknown subcommand",
	"unrecognized command",
	"unrecognized subcommand",
	"unexpected argument",
	"no such command",
}

func containsAnyMarker(text string, markers []string) bool {
	lowered := strings.ToLower(text)
	for _, marker := range markers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// providerDeps holds every seam a diagnosis touches outside its own process:
// running a CLI, resolving one on PATH, and reading the credential files. They
// travel together because a test that describes "signed in, but no credential
// file this app can read" has to control all of them at once.
type providerDeps struct {
	runner     commandRunner
	lookPath   pathLookup
	homeDir    func() (string, error)
	readFile   func(string) ([]byte, error)
	pathExists func(string) bool
}

func defaultDeps() providerDeps {
	return providerDeps{
		runner:   execRunner{},
		lookPath: exec.LookPath,
		homeDir:  os.UserHomeDir,
		readFile: os.ReadFile,
		pathExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}
}

// DiagnoseClaude, DiagnoseCodex and DiagnoseAntigravity report a provider's
// readiness using only what can be determined locally. They are what the
// first-run screen calls: no usage API request, no `/usage`, nothing that
// reaches a provider service, so opening the app on a machine that has never
// been configured contacts nobody. Confirming a connection is a separate,
// user-initiated step - GetXUsage - and these three deliberately cannot do it.
func DiagnoseClaude() Diagnosis {
	diagnosis, _, _ := diagnoseClaude(context.Background(), defaultDeps(), false)
	return diagnosis
}

func DiagnoseCodex() Diagnosis {
	diagnosis, _, _ := diagnoseCodex(context.Background(), defaultDeps(), false)
	return diagnosis
}

func DiagnoseAntigravity() Diagnosis {
	return diagnoseAntigravityLocal(context.Background(), defaultDeps())
}

// diagnoseClaude reports Claude Code's readiness, credentials first.
//
// The credential file - not the CLI - is what this app actually needs, so a
// usable one short-circuits every CLI check: a user who has signed in but whose
// `claude` executable is not on the PATH this app inherited still gets their
// quota. The CLI status command only runs to explain *why* no credential was
// found, which is the one thing the file alone cannot tell us.
//
// ok is true when the caller should perform the usage request; the returned
// credentials carry the token for it. A local credential is never enough to
// report StatusConnected on its own - only a successful usage response is.
func diagnoseClaude(ctx context.Context, deps providerDeps, active bool) (Diagnosis, claudeCredentials, bool) {
	credentials, found := findClaudeCredentials(deps.homeDir, deps.readFile)
	if found {
		if !active {
			details := ""
			if home, err := deps.homeDir(); err == nil {
				details = fmt.Sprintf("Found credentials at %s", filepath.Join(home, filepath.FromSlash(claudeCredentialRelPath)))
			}
			return credentialsFoundDiagnosis(details), credentials, false
		}
		return Diagnosis{}, credentials, true
	}
	return diagnoseClaudeCLI(ctx, deps, active), credentials, false
}

func resolveExecutable(name string, fallbackFunc func(string) (string, bool), deps providerDeps) (string, string) {
	if path, err := deps.lookPath(name); err == nil {
		return path, ""
	}
	if home, err := deps.homeDir(); err == nil {
		if fallback, ok := fallbackFunc(home); ok {
			if deps.pathExists(fallback) {
				return fallback, ""
			}
			return "", fallback
		}
	}
	return "", ""
}

func notFoundDetails(fallback string) string {
	if fallback != "" {
		return fmt.Sprintf("CLI Not Found: %s", fallback)
	}
	return "CLI Not Found: PATH"
}

// The install guide links, one per provider. The same page answers both
// "how do I install this" (not_installed) and "how do I get a newer version"
// (unsupported_cli), so both messages point at it.
const (
	claudeInstallGuideURL      = "https://code.claude.com/docs/ko/quickstart#step-1-install-claude-code"
	codexInstallGuideURL       = "https://learn.chatgpt.com/docs/codex/cli#getting-started"
	antigravityInstallGuideURL = "https://antigravity.google/docs/cli/install"
)

// unsupportedCLIMessage is the message shown for StatusUnsupportedCLI: the
// installed CLI answered, but not in a shape this version understands, so the
// fix is a CLI update rather than a retry - retrying would just run the same
// failing check again against a version that will not change on its own.
func unsupportedCLIMessage(label, guideURL string) string {
	return fmt.Sprintf(`This %s version is not supported. Update to the latest version. <a href="%s">Installation guide</a>`, label, guideURL)
}

// claudeCredentialRelPath and codexCredentialRelPath are the on-disk
// credential file locations, relative to the home directory. Both
// findClaudeCredentials/findCodexCredentials (to read the file) and
// diagnoseClaude/diagnoseCodex (to describe where it was found) resolve
// through the same constant so the two can never drift apart.
const (
	claudeCredentialRelPath = ".claude/.credentials.json"
	codexCredentialRelPath  = ".codex/auth.json"
)

// findCredentials reads a provider's on-disk credential file and reports
// found=false both when the file is absent and when it cannot be parsed or
// carries no usable token - the caller treats those the same way, by falling
// back to the CLI for an explanation, rather than distinguishing them here.
// It is generic over the two JSON shapes Claude and Codex use so that logic
// only has to be written, and tested, once.
func findCredentials[T any](homeDir func() (string, error), readFile func(string) ([]byte, error), relPath string, hasToken func(T) bool) (T, bool) {
	var zero T
	home, err := homeDir()
	if err != nil {
		return zero, false
	}
	data, err := readFile(filepath.Join(home, filepath.FromSlash(relPath)))
	if err != nil {
		return zero, false
	}
	var credentials T
	if err := json.Unmarshal(data, &credentials); err != nil {
		return zero, false
	}
	if !hasToken(credentials) {
		return zero, false
	}
	return credentials, true
}

// diagnoseClaudeCLI is the secondary diagnosis: it runs only when no usable
// credential was found, and exists to tell "never signed in" apart from "signed
// in somewhere this app cannot read".
func diagnoseClaudeCLI(ctx context.Context, deps providerDeps, active bool) Diagnosis {
	path, fallback := resolveExecutable("claude", claudeFallbackPath, deps)
	if path == "" {
		return Diagnosis{
			Status:  StatusNotInstalled,
			Message: `Install Claude Code CLI (<code>claude</code>) and log in to monitor your quota. <a href="` + claudeInstallGuideURL + `">Installation guide</a>`,
			Details: technicalDetails(notFoundDetails(fallback)),
		}
	}

	ctx, cancel := context.WithTimeout(ctx, statusCommandTimeout)
	defer cancel()
	result, err := deps.runner.run(ctx, path, "auth", "status", "--json")
	if err != nil {
		return Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not read the Claude Code login state. Try again.",
			Details: technicalDetails(err.Error() + " " + result.Stderr),
		}
	}

	// Parse before looking at the exit code. The logged-in path was verified to
	// answer with JSON and exit 0, but the logged-out exit code is still an open
	// item in the release gate, so a machine-readable answer is trusted whatever
	// the process returned. Only output we cannot read at all falls through to
	// the exit-code based classification below. Just `loggedIn` is read; the
	// same response carries email, orgId and orgName, which never leave here.
	var status struct {
		LoggedIn *bool `json:"loggedIn"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &status); err == nil && status.LoggedIn != nil {
		if !*status.LoggedIn {
			return Diagnosis{
				Status:  StatusLoginRequired,
				Message: "Log in to Claude Code to view quota information.",
				Details: technicalDetails(result.Stdout),
			}
		}
		if !active {
			return loggedInLocallyDiagnosis("Claude Code", path)
		}
		return unsupportedCredentialSourceDiagnosis("Claude Code")
	}

	return unreadableStatusDiagnosis("Claude Code", claudeInstallGuideURL, result)
}

// diagnoseCodex mirrors diagnoseClaude. The difference is the secondary
// command: `codex login status` prints prose - on the verified version, to
// stderr with an empty stdout - so its result is read from the exit code alone
// and the output is only ever kept as redacted technical detail.
func diagnoseCodex(ctx context.Context, deps providerDeps, active bool) (Diagnosis, codexAuth, bool) {
	credentials, found := findCodexCredentials(deps.homeDir, deps.readFile)
	if found {
		if !active {
			details := ""
			if home, err := deps.homeDir(); err == nil {
				details = fmt.Sprintf("Found credentials at %s", filepath.Join(home, filepath.FromSlash(codexCredentialRelPath)))
			}
			return credentialsFoundDiagnosis(details), credentials, false
		}
		return Diagnosis{}, credentials, true
	}
	return diagnoseCodexCLI(ctx, deps, active), credentials, false
}

func diagnoseCodexCLI(ctx context.Context, deps providerDeps, active bool) Diagnosis {
	path, fallback := resolveExecutable("codex", codexFallbackPath, deps)
	if path == "" {
		return Diagnosis{
			Status:  StatusNotInstalled,
			Message: `Install the Codex CLI (<code>codex</code>) and log in to monitor your quota. <a href="` + codexInstallGuideURL + `">Installation guide</a>`,
			Details: technicalDetails(notFoundDetails(fallback)),
		}
	}

	ctx, cancel := context.WithTimeout(ctx, statusCommandTimeout)
	defer cancel()
	result, err := deps.runner.run(ctx, path, "login", "status")
	if err != nil {
		return Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not read the Codex login state. Try again.",
			Details: technicalDetails(err.Error() + " " + result.Stderr),
		}
	}

	output := result.Stdout + " " + result.Stderr
	if containsAnyMarker(output, unsupportedCLIMarkers) {
		return Diagnosis{
			Status:  StatusUnsupportedCLI,
			Message: unsupportedCLIMessage("Codex", codexInstallGuideURL),
			Details: technicalDetails(output),
		}
	}
	if result.ExitCode != 0 {
		// The exact logged-out exit code is still to be captured as a fixture
		// (see the release gate), so anything non-zero is read as logged out
		// rather than as a broken CLI: telling a logged-out user to log in is
		// recoverable, while hiding that guidance behind a compatibility error
		// is not.
		return Diagnosis{
			Status:  StatusLoginRequired,
			Message: "Log in to Codex to view quota information.",
			Details: technicalDetails(output),
		}
	}
	if !active {
		return loggedInLocallyDiagnosis("Codex", path)
	}
	return unsupportedCredentialSourceDiagnosis("Codex")
}

// diagnoseAntigravity reports Antigravity's readiness. agy is the only
// provider whose CLI is genuinely required, because the usage lookup *is* an
// agy command - there is no credential file to fall back to. It also has no
// local sign-in command (`agy auth status` does not exist through 1.1.28), so
// the network gate sits earlier here: `--version` is checked first, network-
// free, for every caller including an active one - a broken or incompatible
// CLI is then reported without ever attempting the heavier `/usage` request,
// and the version state itself only comes back with `/usage`.
func diagnoseAntigravity(ctx context.Context, runner commandRunner, agyPath string, active bool) (Diagnosis, bool) {
	ctx, cancel := context.WithTimeout(ctx, statusCommandTimeout)
	defer cancel()
	result, err := runner.run(ctx, agyPath, "--version")
	if err != nil {
		return Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not run the Antigravity CLI. Try again.",
			Details: technicalDetails(err.Error() + " " + result.Stderr),
		}, false
	}
	if result.ExitCode != 0 {
		return Diagnosis{
			Status:  StatusUnsupportedCLI,
			Message: unsupportedCLIMessage("Antigravity CLI", antigravityInstallGuideURL),
			Details: technicalDetails(result.Stdout + " " + result.Stderr),
		}, false
	}

	if !active {
		return Diagnosis{
			Status:  StatusAuthCheckRequired,
			Message: "Antigravity CLI found. Connect to verify usage.",
			Details: technicalDetails(fmt.Sprintf("Found agy (%s) at %s", strings.TrimSpace(result.Stdout), agyPath)),
		}, false
	}
	return Diagnosis{}, true
}

// credentialsFoundDiagnosis is where a provider rests when its credential file
// is readable but the user has not switched it on: enough is known to offer a
// connection check, and deliberately not enough to claim a working connection,
// since confirming that would take the very request this state exists to avoid.
func credentialsFoundDiagnosis(details string) Diagnosis {
	return Diagnosis{
		Status:  StatusAuthCheckRequired,
		Message: "Credentials found. Connect to verify usage.",
		Details: technicalDetails(details),
	}
}

// loggedInLocallyDiagnosis is the same waiting state reached the other way: no
// credential file this app can read, but the CLI reports a local session.
func loggedInLocallyDiagnosis(label string, path string) Diagnosis {
	details := fmt.Sprintf("%s CLI found and reported an active session.", label)
	if path != "" {
		details = fmt.Sprintf("Found %s at %s with an active session.", label, path)
	}
	return Diagnosis{
		Status:  StatusAuthCheckRequired,
		Message: "Logged in locally. Connect to verify usage.",
		Details: technicalDetails(details),
	}
}

// unsupportedCredentialSourceDiagnosis covers the gap the CLI reveals: it
// considers itself logged in, but the session is not in a place this app reads.
// The guidance is compatibility help rather than "log in again", because
// logging in again generally writes the credential right back to the same
// unsupported place.
func unsupportedCredentialSourceDiagnosis(label string) Diagnosis {
	return Diagnosis{
		Status:  StatusUsageUnavailable,
		Reason:  ReasonUnsupportedCredentialSource,
		Message: label + " is logged in, but AI Gauge cannot access a supported local credential source.",
	}
}

// unreadableStatusDiagnosis classifies a status command whose output we could
// not interpret. An exit code of 0 with unreadable output means the CLI answered
// in a shape this version does not know, which is a compatibility problem; a
// non-zero exit is left as a temporary error rather than as "logged out",
// because the action plan forbids inferring a logout from an unrecognized
// failure.
func unreadableStatusDiagnosis(label, guideURL string, result commandResult) Diagnosis {
	output := result.Stdout + " " + result.Stderr
	details := technicalDetails(output)
	if containsAnyMarker(output, unsupportedCLIMarkers) || result.ExitCode == 0 {
		return Diagnosis{
			Status:  StatusUnsupportedCLI,
			Message: unsupportedCLIMessage(label, guideURL),
			Details: details,
		}
	}
	return Diagnosis{
		Status:  StatusTemporaryError,
		Message: "Could not read the " + label + " login state. Try again.",
		Details: details,
	}
}

// usageFailureDiagnosis maps a usage request that failed after a credential was
// found. A 401 or 403 is precisely the case a local credential cannot reveal -
// the stored token expired or was revoked server-side - and it sends the card
// back to login guidance instead of leaving a stale "connected".
func usageFailureDiagnosis(label string, err error) Diagnosis {
	details := ""
	if err != nil {
		details = technicalDetails(err.Error())
	}
	switch httpStatusCode(err) {
	case 401, 403:
		return Diagnosis{
			Status:  StatusLoginRequired,
			Message: "Your " + label + " session expired. Log in again to view quota information.",
			Details: details,
		}
	}
	return Diagnosis{
		Status:  StatusTemporaryError,
		Message: "Could not reach " + label + " right now. Retry in a moment.",
		Details: details,
	}
}

// usageUnreadableDiagnosis covers a usage response that arrived but could not be
// turned into gauges - a schema this version does not support, or an account
// that reports no usage window at all. They share a status and differ in Reason
// because only one of them is fixed by updating something.
func usageUnreadableDiagnosis(label string, reason Reason, err error) Diagnosis {
	message := "Quota information is not available for this " + label + " account."
	if reason == ReasonUnsupportedResponse {
		message = label + " returned usage data this version cannot read. Update AI Gauge or the CLI."
	}
	details := ""
	if err != nil {
		details = technicalDetails(err.Error())
	}
	return Diagnosis{
		Status:  StatusUsageUnavailable,
		Reason:  reason,
		Message: message,
		Details: details,
	}
}

// httpStatusCode recovers the HTTP status from a fetchAuthorizedJSON failure,
// returning 0 for a transport error that never reached a response. A missing
// code lands on the safe StatusTemporaryError branch above.
func httpStatusCode(err error) int {
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode
	}
	return 0
}
