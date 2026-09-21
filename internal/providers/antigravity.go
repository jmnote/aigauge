package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Disable agy auto-update to prevent console window flashes:
// https://antigravity.google/docs/cli/troubleshooting/#resolution-3
const agyDisableAutoUpdateEnv = "AGY_CLI_DISABLE_AUTO_UPDATE=true"

func runAgy(ctx context.Context, runner commandRunner, agyPath string, args ...string) (commandResult, error) {
	return runner.run(ctx, []string{agyDisableAutoUpdateEnv}, agyPath, args...)
}

// antigravityInstallGuideURL answers both "how do I install this" and "how do
// I get a newer version".
const antigravityInstallGuideURL = "https://antigravity.google/docs/cli/install/"

// antigravityAuthMarkers are the phrases that let a failed agy command be read
// as "signed out" rather than "something went wrong". The list is
// deliberately short: an unrecognized failure must fall through to a
// temporary error, since telling a signed-in user they are signed out is the
// worse mistake.
var antigravityAuthMarkers = []string{
	"not logged in",
	"not signed in",
	"unauthorized",
	"authentication required",
	"authentication failed",
	"please log in",
	"please sign in",
	"login required",
	"401",
}

const (
	// antigravityUsageTimeout bounds the whole `/usage` run. It stays well above
	// the 30s --print-timeout handed to agy itself so the CLI gets the chance to
	// report its own timeout before this one kills it.
	antigravityUsageTimeout = 45 * time.Second

	// antigravityModelsTimeout bounds the optional `agy models` follow-up, which
	// only ever runs to disambiguate a failure that already happened.
	antigravityModelsTimeout = 30 * time.Second
)

// EnsureAntigravityCLI verifies that agy is installed and authenticated.
// The usage command itself is intentionally kept on the CLI path: unlike the
// local Hub APIs, `agy -p /usage` includes the weekly quota information shown
// by the CLI.
func EnsureAntigravityCLI() Diagnosis {
	deps := defaultDeps()
	agyPath, notInstalled, found := findAgy(deps)
	if !found {
		return notInstalled
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusCommandTimeout)
	defer cancel()
	return checkAntigravityModels(ctx, deps.runner, agyPath, "agy authentication confirmed.")
}

func checkAntigravityModels(ctx context.Context, runner commandRunner, agyPath, successMessage string) Diagnosis {
	result, err := runAgy(ctx, runner, agyPath, "models")
	if err != nil {
		return Diagnosis{Status: StatusTemporaryError, Message: "Could not check Antigravity models.", Details: technicalDetails(err.Error())}
	}
	details := strings.TrimSpace(result.Stderr)
	if details == "" {
		details = strings.TrimSpace(result.Stdout)
	}
	if result.ExitCode != 0 {
		if containsAntigravityAuthMarker(details) || strings.Contains(strings.ToLower(details), "sign in") {
			return Diagnosis{Status: StatusLoginRequired, Message: "Sign in with agy first, then try again.", Details: technicalDetails(details)}
		}
		if containsAnyMarker(details, unsupportedCLIMarkers) {
			return Diagnosis{Status: StatusUnsupportedCLI, Message: unsupportedCLIMessage("Antigravity CLI", antigravityInstallGuideURL), Details: technicalDetails(details)}
		}
		return Diagnosis{Status: StatusTemporaryError, Message: "Could not check Antigravity models.", Details: technicalDetails(details)}
	}
	return Diagnosis{Status: StatusConnected, Message: successMessage}
}

func containsAntigravityAuthMarker(message string) bool {
	lower := strings.ToLower(message)
	for _, marker := range antigravityAuthMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// ParseAntigravityUsage unmarshals agy's raw `/usage` stdout and converts it
// into AI Gauge's own shape - see AntigravityUsage's doc comment.
func ParseAntigravityUsage(data []byte) (AntigravityUsage, error) {
	var response agyUsageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return AntigravityUsage{}, err
	}

	usage := AntigravityUsage{
		Description: response.Command.Data.Description,
		Raw:         append(json.RawMessage(nil), data...),
	}
	for _, group := range response.Command.Data.Groups {
		g := AntigravityUsageGroup{DisplayName: group.Name, Description: group.Description}
		for _, bucket := range group.Buckets {
			g.Buckets = append(g.Buckets, AntigravityUsageBucket{
				BucketID:          bucket.ID,
				DisplayName:       bucket.Name,
				Window:            bucket.Window,
				Description:       bucket.Description,
				RemainingFraction: bucket.RemainingFraction,
				ResetTime:         bucket.ResetTime,
			})
		}
		usage.Groups = append(usage.Groups, g)
	}
	return usage, nil
}

var antigravityWindowOrder = map[string]int{"5h": 0, "24h": 1, "weekly": 2}
var antigravityWindowLabels = map[string]string{"5h": "5h", "weekly": "7d"}

func antigravityOrder(window string) int {
	if order, ok := antigravityWindowOrder[window]; ok {
		return order
	}
	return len(antigravityWindowOrder)
}

// ToDisplay resolves each bucket's window into a display label and a stable
// 5h/24h/weekly order, and its fraction into a percentage.
func (u AntigravityUsage) ToDisplay() DisplayUsage {
	display := DisplayUsage{FetchedAt: u.FetchedAt, DiagnosisFields: u.DiagnosisFields}
	if u.Status != StatusConnected {
		return display
	}
	for _, group := range u.Groups {
		buckets := append([]AntigravityUsageBucket(nil), group.Buckets...)
		sort.SliceStable(buckets, func(i, j int) bool {
			return antigravityOrder(buckets[i].Window) < antigravityOrder(buckets[j].Window)
		})
		displayGroup := DisplayUsageGroup{Name: group.DisplayName}
		for _, bucket := range buckets {
			label := bucket.DisplayName
			if l, ok := antigravityWindowLabels[bucket.Window]; ok {
				label = l
			}
			displayGroup.Buckets = append(displayGroup.Buckets, DisplayUsageBucket{
				Label:     label,
				Remaining: bucket.RemainingFraction * 100,
				ResetTime: bucket.ResetTime,
			})
		}
		display.Groups = append(display.Groups, displayGroup)
	}
	display.Status = StatusConnected
	return display
}

// GetAntigravityUsage runs the real, agy-CLI-backed usage lookup. tokenKey is
// accepted only to match the other providers' per-instance signature: agy
// manages a single local session of its own, so every Antigravity instance
// reflects that same session rather than a credential AI Gauge stores itself.
func GetAntigravityUsage(tokenKey string) AntigravityUsage {
	return getAntigravityUsage(context.Background(), defaultDeps(), tokenKey, true)
}

// FetchAntigravityRawUsage returns agy's unconverted `/usage` stdout. Used by
// hack/fixtures/fixtures.go to capture the CLI's actual response shape for
// fixture development.
func FetchAntigravityRawUsage(_ string) ([]byte, error) {
	deps := defaultDeps()
	agyPath, notInstalled, found := findAgy(deps)
	if !found {
		return nil, errors.New(notInstalled.Message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), antigravityUsageTimeout)
	defer cancel()
	modelsCtx, cancel := context.WithTimeout(ctx, antigravityModelsTimeout)
	defer cancel()
	models := checkAntigravityModels(modelsCtx, deps.runner, agyPath, "agy authentication confirmed.")
	if models.Status != StatusConnected {
		return nil, errors.New(models.Message)
	}
	result, err := runAgy(ctx, deps.runner, agyPath, "-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("agy exited %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return []byte(result.Stdout), nil
}

// findAgy resolves the agy executable: PATH first, then the platform's
// fallback install location. The raw lookup failure is kept under Details for
// transparency, while Message provides clear guidance.
func findAgy(deps providerDeps) (string, Diagnosis, bool) {
	path, fallback := resolveExecutable("agy", antigravityFallbackPath, deps)
	if path == "" {
		return "", Diagnosis{
			Status:  StatusNotInstalled,
			Message: `Install the Antigravity CLI (<code>agy</code>) and log in to monitor your quota. <a href="` + antigravityInstallGuideURL + `">Installation guide</a>`,
			Details: technicalDetails(notFoundDetails(fallback)),
		}, false
	}
	return path, Diagnosis{}, true
}

// diagnoseAntigravityLocal answers with what can be known offline: whether agy
// is installed and whether its version is supported. It never runs `/usage`,
// which makes it the safe call for the onboarding screen. tokenKey is unused
// (see GetAntigravityUsage) but kept to match DiagnoseClaude/DiagnoseCodex's
// per-instance signature.
func diagnoseAntigravityLocal(ctx context.Context, deps providerDeps, _ string) Diagnosis {
	agyPath, notInstalled, ok := findAgy(deps)
	if !ok {
		return notInstalled
	}
	diagnosis, _ := diagnoseAntigravity(ctx, deps.runner, agyPath, false)
	return diagnosis
}

// diagnoseAntigravity reports Antigravity's readiness. agy is the only
// provider whose CLI is genuinely required, because the usage lookup *is* an
// agy command - there is no credential file to fall back to, and reading
// agy's own local token store to call Google's API directly is deliberately
// avoided (see docs/privacy-policy.md): Google's terms treat that as a
// third-party tool using an Antigravity login. It also has no local sign-in
// command (`agy auth status` does not exist through 1.1.28), so the network
// gate sits earlier here: `--version` is checked first, network-free, for
// every caller including an active one - a broken or incompatible CLI is then
// reported without ever attempting the heavier `/usage` request, and the
// version state itself only comes back with `/usage`.
func diagnoseAntigravity(ctx context.Context, runner commandRunner, agyPath string, active bool) (Diagnosis, bool) {
	ctx, cancel := context.WithTimeout(ctx, statusCommandTimeout)
	defer cancel()
	result, err := runAgy(ctx, runner, agyPath, "--version")
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

// getAntigravityUsage backs both "Check connection" and the recurring poll.
// It checks `--version` first (via diagnoseAntigravity) even when active, so
// a broken or incompatible CLI is reported without ever attempting the
// heavier `/usage` request - see diagnoseAntigravity's doc comment.
func getAntigravityUsage(ctx context.Context, deps providerDeps, _ string, active bool) AntigravityUsage {
	usage := AntigravityUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	agyPath, notInstalled, found := findAgy(deps)
	if !found {
		usage.applyDiagnosis(notInstalled)
		return usage
	}

	if !active {
		diagnosis, ok := diagnoseAntigravity(ctx, deps.runner, agyPath, false)
		if !ok {
			usage.applyDiagnosis(diagnosis)
			return usage
		}
	}

	// `models` is the authoritative, lightweight authentication check. Run it
	// before `/usage` so a signed-out session is reported without launching the
	// heavier interactive prompt command.
	modelsCtx, cancel := context.WithTimeout(ctx, antigravityModelsTimeout)
	defer cancel()
	models := checkAntigravityModels(modelsCtx, deps.runner, agyPath, "agy authentication confirmed.")
	if models.Status != StatusConnected {
		usage.applyDiagnosis(models)
		return usage
	}

	usageCtx, cancel := context.WithTimeout(ctx, antigravityUsageTimeout)
	defer cancel()
	result, runErr := runAgy(usageCtx, deps.runner, agyPath,
		"-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	if runErr != nil {
		usage.applyDiagnosis(Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not reach Antigravity right now. Retry in a moment.",
			Details: technicalDetails(runErr.Error() + " " + result.Stderr),
		})
		return usage
	}

	parsed, diagnosis := classifyAntigravityUsage(ctx, deps, agyPath, result)
	if diagnosis.Status == StatusConnected {
		usage.Groups = parsed.Groups
		usage.Description = parsed.Description
	}
	usage.applyDiagnosis(diagnosis)
	return usage
}

// classifyAntigravityUsage turns one `/usage` run into a state. `/usage` is the
// single path that proves sign-in and usage access at once, so a clean run with
// at least one group is the only thing that yields StatusConnected.
func classifyAntigravityUsage(ctx context.Context, deps providerDeps, agyPath string, result commandResult) (AntigravityUsage, Diagnosis) {
	if result.ExitCode == 0 {
		parsed, err := ParseAntigravityUsage([]byte(result.Stdout))
		if err == nil && len(parsed.Groups) > 0 {
			return parsed, Diagnosis{Status: StatusConnected}
		}
		// Exit 0 means agy authenticated and answered; we just cannot show it.
		reason := ReasonNoUsageData
		if err != nil {
			reason = ReasonUnsupportedResponse
		}
		return AntigravityUsage{}, usageUnreadableDiagnosis("Antigravity", reason, err)
	}

	output := result.Stdout + " " + result.Stderr
	if containsAnyMarker(output, antigravityAuthMarkers) {
		return AntigravityUsage{}, Diagnosis{
			Status:  StatusLoginRequired,
			Message: "Log in to the Antigravity CLI to view quota information.",
			Details: technicalDetails(output),
		}
	}
	if containsAnyMarker(output, unsupportedCLIMarkers) {
		return AntigravityUsage{}, Diagnosis{
			Status:  StatusUnsupportedCLI,
			Message: unsupportedCLIMessage("Antigravity CLI", antigravityInstallGuideURL),
			Details: technicalDetails(output),
		}
	}
	return AntigravityUsage{}, classifyAntigravityWithModels(ctx, deps, agyPath, output)
}

// classifyAntigravityWithModels is the optional secondary diagnostic. It runs
// only for a `/usage` failure we could not read, and only to answer one
// question: was that an authentication problem, or a usage response this
// version cannot handle? A working `agy models` proves the session is fine and
// narrows the failure to the response itself.
func classifyAntigravityWithModels(ctx context.Context, deps providerDeps, agyPath, usageOutput string) Diagnosis {
	modelsCtx, cancel := context.WithTimeout(ctx, antigravityModelsTimeout)
	defer cancel()
	result, err := runAgy(modelsCtx, deps.runner, agyPath, "models")
	if err != nil {
		return Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not reach Antigravity right now. Retry in a moment.",
			Details: technicalDetails(usageOutput),
		}
	}

	// The output is not structured JSON, so nothing here depends on model names:
	// a zero exit plus at least one non-empty stdout line is the whole test.
	// Progress chatter like "Fetching available models..." goes to stderr and is
	// kept apart from this check.
	if result.ExitCode == 0 && hasListRow(result.Stdout) {
		return usageUnreadableDiagnosis("Antigravity", ReasonUnsupportedResponse, errors.New(usageOutput))
	}
	if containsAnyMarker(result.Stdout+" "+result.Stderr, antigravityAuthMarkers) {
		return Diagnosis{
			Status:  StatusLoginRequired,
			Message: "Log in to the Antigravity CLI to view quota information.",
			Details: technicalDetails(usageOutput),
		}
	}
	return Diagnosis{
		Status:  StatusTemporaryError,
		Message: "Could not reach Antigravity right now. Retry in a moment.",
		Details: technicalDetails(usageOutput),
	}
}

func hasListRow(stdout string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}
