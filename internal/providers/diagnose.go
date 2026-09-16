package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jmnote/aigauge/internal/auth"
)

// statusCommandTimeout bounds a local CLI status command. These commands read
// on-disk credential state and answer immediately; anything slower is a hung
// process, not a slow answer, and the provider card should say "try again"
// rather than sit on a spinner.
const statusCommandTimeout = 10 * time.Second

// unsupportedCLIMarkers are the common ways a CLI-backed provider rejects a
// command or option it does not implement. Command-line parsers normally
// return a non-zero exit code for these errors, so recognize them before the
// generic login or temporary-error branches.
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

// providerDeps holds the seams a diagnosis or usage fetch touches: retrieving
// stored tokens, checking if legacy CLI credentials exist for import,
// obtaining an access token that has been refreshed if it was expired, and -
// for the one remaining CLI-backed provider (Antigravity, via agy) - running
// and resolving that CLI.
type providerDeps struct {
	getToken  func(string) (*auth.Token, error)
	canImport func(string) bool

	// getValidAccessToken refreshes an expired token (see auth.
	// GetValidAccessToken) before a usage fetch uses it. It is nil in the
	// diagnose-only tests, which never look at it: falling back to the
	// credential's own access token when it is nil is deliberate rather than
	// a special case, so those tests need no change to keep passing.
	getValidAccessToken func(ctx context.Context, cfgType, tokenKey string) (string, error)

	runner     commandRunner
	lookPath   pathLookup
	homeDir    func() (string, error)
	readFile   func(string) ([]byte, error)
	pathExists func(string) bool
}

func defaultDeps() providerDeps {
	return providerDeps{
		getToken:            auth.GetToken,
		canImport:           auth.CanImportLegacy,
		getValidAccessToken: auth.GetValidAccessToken,
		runner:              execRunner{},
		lookPath:            exec.LookPath,
		homeDir:             os.UserHomeDir,
		readFile:            os.ReadFile,
		pathExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}
}

// resolveExecutable resolves a CLI's executable path: PATH first, then the
// platform's fallback install location. The raw lookup failure is kept
// alongside so a "not installed" diagnosis can say where it looked.
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

// unsupportedCLIMessage is the message shown for StatusUnsupportedCLI: the
// installed CLI answered, but not in a shape this version understands, so the
// fix is a CLI update rather than a retry - retrying would just run the same
// failing check again against a version that will not change on its own.
func unsupportedCLIMessage(label, guideURL string) string {
	return fmt.Sprintf(`This %s version is not supported. Update to the latest version. <a href="%s">Installation guide</a>`, label, guideURL)
}

// authorizedAccessToken returns the refreshed token when needed. A refresh
// failure stays a retryable service error instead of sending an expired token
// and misclassifying the resulting 401 as a logged-out session.
func authorizedAccessToken(ctx context.Context, deps providerDeps, cfgType, tokenKey, fallback string) (string, error) {
	if deps.getValidAccessToken == nil {
		return fallback, nil
	}
	fresh, err := deps.getValidAccessToken(ctx, cfgType, tokenKey)
	if err != nil {
		return "", err
	}
	if fresh == "" {
		return "", fmt.Errorf("provider returned an empty access token")
	}
	return fresh, nil
}

// DiagnoseClaude, DiagnoseCodex and DiagnoseAntigravity report a provider's
// readiness locally without making network calls. tokenKey identifies the
// provider *instance* (not just its type) in the auth token store, so two
// instances of the same type never see each other's credentials.
func DiagnoseClaude(tokenKey string) Diagnosis {
	diagnosis, _, _ := diagnoseClaude(context.Background(), defaultDeps(), tokenKey, false)
	return diagnosis
}

func DiagnoseCodex(tokenKey string) Diagnosis {
	diagnosis, _, _ := diagnoseCodex(context.Background(), defaultDeps(), tokenKey, false)
	return diagnosis
}

func DiagnoseAntigravity(tokenKey string) Diagnosis {
	return diagnoseAntigravityLocal(context.Background(), defaultDeps(), tokenKey)
}

// diagnoseClaude reports Claude's readiness using stored tokens first.
func diagnoseClaude(_ context.Context, deps providerDeps, tokenKey string, active bool) (Diagnosis, claudeCredentials, bool) {
	if deps.getToken != nil {
		if tok, err := deps.getToken(tokenKey); err == nil && tok != nil && tok.AccessToken != "" {
			creds := claudeCredentials{}
			creds.ClaudeAiOauth.AccessToken = tok.AccessToken
			creds.ClaudeAiOauth.SubscriptionType = tok.Extra.Plan
			if !active {
				return credentialsFoundDiagnosis("Credentials found. Connect to verify usage."), creds, false
			}
			return Diagnosis{}, creds, true
		}
	}
	canImport := false
	if deps.canImport != nil {
		canImport = deps.canImport("claude")
	}
	return Diagnosis{
		Status:    StatusLoginRequired,
		Message:   "Connect to Claude to view quota information.",
		CanImport: canImport,
	}, claudeCredentials{}, false
}

// diagnoseCodex reports Codex's readiness using stored tokens first.
func diagnoseCodex(_ context.Context, deps providerDeps, tokenKey string, active bool) (Diagnosis, codexAuth, bool) {
	if deps.getToken != nil {
		if tok, err := deps.getToken(tokenKey); err == nil && tok != nil && tok.AccessToken != "" {
			creds := codexAuth{}
			creds.Tokens.AccessToken = tok.AccessToken
			if !active {
				return credentialsFoundDiagnosis("Credentials found. Connect to verify usage."), creds, false
			}
			return Diagnosis{}, creds, true
		}
	}
	canImport := false
	if deps.canImport != nil {
		canImport = deps.canImport("codex")
	}
	return Diagnosis{
		Status:    StatusLoginRequired,
		Message:   "Connect to Codex to view quota information.",
		CanImport: canImport,
	}, codexAuth{}, false
}

// credentialsFoundDiagnosis is where a provider rests when its credentials
// exist in the store but the card is inactive.
func credentialsFoundDiagnosis(details string) Diagnosis {
	return Diagnosis{
		Status:  StatusAuthCheckRequired,
		Message: "Credentials found. Connect to verify usage.",
		Details: technicalDetails(details),
	}
}

// usageFailureDiagnosis maps a usage request failure to an actionable state.
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

// usageUnreadableDiagnosis covers a usage response that arrived but could not be parsed.
func usageUnreadableDiagnosis(label string, reason Reason, err error) Diagnosis {
	message := "Quota information is not available for this " + label + " account."
	if reason == ReasonUnsupportedResponse {
		message = label + " returned usage data this version cannot read. Update AI Gauge."
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

// httpStatusCode recovers the HTTP status from a fetchAuthorizedJSON failure.
func httpStatusCode(err error) int {
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode
	}
	return 0
}
