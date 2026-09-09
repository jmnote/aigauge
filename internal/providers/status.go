package providers

import (
	"regexp"
	"strings"
)

// Status is the structured readiness code a provider reports about itself.
// It exists so the frontend never has to guess what an error string meant:
// internal/providers decides, internal/app forwards the code verbatim over
// the Wails binding, and the frontend only picks the wording and controls
// for the code it was given. These values are what the Store certification
// notes describe, so they are part of the app's external contract - renaming one
// means updating the provider cards and the Store listing together.
type Status string

const (
	// StatusConnected means the sign-in was confirmed *and* the usage lookup
	// succeeded. A locally stored credential alone is never enough: both
	// Claude and Codex can report a local login whose token the server has
	// since rejected, so this is only set after real usage data comes back.
	StatusConnected Status = "connected"

	// StatusNotInstalled means the CLI executable was not found in PATH or in
	// the supported default install location.
	StatusNotInstalled Status = "not_installed"

	// StatusAuthCheckRequired means the CLI is installed and whatever could be
	// determined locally has been, but the network verification that would
	// settle the provider's real state has not been requested yet. This is the
	// state that keeps a disabled provider from making any outbound request on
	// startup, which is why it is a first-class code rather than an error.
	StatusAuthCheckRequired Status = "auth_check_required"

	// StatusLoginRequired means the CLI is present but not logged in - either
	// its own status command said so, or a usage request was rejected as
	// unauthorized despite a local credential being present.
	StatusLoginRequired  Status = "login_required"
	StatusSignInRequired Status = StatusLoginRequired

	// StatusUsageUnavailable means authentication worked but the provider
	// returned no usable quota data.
	StatusUsageUnavailable Status = "usage_unavailable"

	// StatusTemporaryError means a network or service failure that is expected
	// to resolve on its own. Anything unrecognized lands here rather than in
	// StatusLoginRequired: telling a logged-in user they are logged out is a
	// worse failure than asking them to retry.
	StatusTemporaryError Status = "temporary_error"

	// StatusUnsupportedCLI means the installed CLI version does not provide the
	// command or option this app needs.
	StatusUnsupportedCLI Status = "unsupported_cli"
)

// NeedsUserAction reports whether status is one of the expected, user-fixable
// setup states rather than a failure. Callers use it to keep these states out
// of failure counters, automatic retry loops, and red error styling - the
// behavior that made the previous certification submission look broken on a
// clean review device.
func (s Status) NeedsUserAction() bool {
	return s == StatusNotInstalled || s == StatusAuthCheckRequired || s == StatusLoginRequired || s == StatusSignInRequired
}

// Reason narrows a status whose recovery differs case by case. Only
// StatusUsageUnavailable needs it today: "your plan reports no quota windows",
// "your credentials live somewhere this app cannot read", and "the response
// did not match the schema we support" all present as unavailable usage but
// need different guidance, and offering one generic "check again" for all
// three leaves the middle case with no way out.
type Reason string

const (
	// ReasonNoUsageData means the account authenticated but reported no usage
	// window to display.
	ReasonNoUsageData Reason = "no_usage_data"

	// ReasonUnsupportedCredentialSource means the CLI considers itself signed
	// in but stores the session somewhere this app does not read - an OS
	// keychain, for instance. Re-signing in usually does not move it, so the
	// guidance is compatibility help rather than "sign in again".
	ReasonUnsupportedCredentialSource Reason = "unsupported_credential_source"

	// ReasonUnsupportedResponse means the provider answered in a shape this
	// version cannot parse, which an app or CLI update may fix.
	ReasonUnsupportedResponse Reason = "unsupported_response"
)

// Diagnosis is a provider's structured answer to "what is your state and what
// should the user do next". Message is the always-visible next action; Details
// is the redacted, length-capped text that belongs behind the card's
// "Technical details" disclosure and never in the primary message.
//
// Message carries only the explanatory sentence for the card body. The status
// badge is rendered from Status alone, because the badge has to stay legible in
// a 250px window - "Check connection" fits there, "Credentials found - check
// connection" does not.
type Diagnosis struct {
	Status  Status `json:"status"`
	Reason  Reason `json:"reason,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// maxDetailLength caps how much command output can reach the UI. CLI failures
// can print stack traces or wrapped JSON bodies, and the provider card only
// ever shows a short excerpt behind a disclosure.
const maxDetailLength = 400

// redactionPatterns strip the values that provider CLIs are known to print
// alongside their status - the account e-mail, OAuth/API tokens, and the UUIDs
// used for organization and project identifiers. None of these may reach the UI
// or a log line, and because a status
// command's output format can change between CLI versions, redaction runs over
// every string we pass on rather than only over the fields we recognize today.
var redactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`),
	regexp.MustCompile(`(?i)\bbearer\s+\S+`),
	regexp.MustCompile(`(?i)\b(?:sk|pk|oauth|tok|key|ghp|gho)[-_][A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`),
	// Catch-all for opaque credentials that match none of the shapes above:
	// an unbroken 32+ character run of token alphabet. Ordinary prose and file
	// paths break well before that, but access tokens rarely do.
	regexp.MustCompile(`\b[A-Za-z0-9_-]{32,}\b`),
}

// redact removes credential-shaped substrings from text. It is deliberately
// applied to whole command output rather than to individual parsed fields,
// since the point is to survive output we did not anticipate.
func redact(text string) string {
	for _, pattern := range redactionPatterns {
		text = pattern.ReplaceAllString(text, "[redacted]")
	}
	return text
}

// technicalDetails turns raw command output into something safe to show:
// redacted, whitespace-collapsed, and truncated. It returns an empty string
// for output that carried nothing but whitespace so the caller can omit the
// disclosure entirely instead of rendering an empty one.
func technicalDetails(text string) string {
	text = strings.TrimSpace(redact(text))
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > maxDetailLength {
		// Cut on a rune boundary so a multi-byte character at the limit does
		// not become a replacement character in the UI.
		cut := maxDetailLength
		for cut > 0 && !isRuneStart(text[cut]) {
			cut--
		}
		text = strings.TrimSpace(text[:cut]) + "..."
	}
	return text
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
