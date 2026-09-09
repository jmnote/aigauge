package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AntigravityUsage struct {
	Groups    []AntigravityUsageGroup `json:"groups"`
	FetchedAt string                  `json:"fetchedAt"`
	Error     string                  `json:"error,omitempty"`

	// See ClaudeUsage: the structured diagnosis, with Error kept in step for the
	// frontend that has not migrated to Status yet.
	Status  Status `json:"status,omitempty"`
	Reason  Reason `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"details,omitempty"`
}

func (u *AntigravityUsage) applyDiagnosis(diagnosis Diagnosis) {
	u.Status = diagnosis.Status
	u.Reason = diagnosis.Reason
	u.Message = diagnosis.Message
	u.Details = diagnosis.Details
	u.Error = diagnosis.Message
}

type AntigravityUsageGroup struct {
	Name    string                   `json:"name"`
	Buckets []AntigravityUsageBucket `json:"buckets"`
}

type AntigravityUsageBucket struct {
	Name      string  `json:"name"`
	Window    string  `json:"window"`
	Remaining float64 `json:"remaining"`
	ResetTime string  `json:"resetTime"`
}

type agyUsageResponse struct {
	Command struct {
		Data struct {
			Groups []struct {
				Name    string `json:"name"`
				Buckets []struct {
					Name              string  `json:"name"`
					Window            string  `json:"window"`
					RemainingFraction float64 `json:"remaining_fraction"`
					ResetTime         string  `json:"reset_time"`
				} `json:"buckets"`
			} `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

func parseAntigravityUsage(output []byte) (AntigravityUsage, error) {
	var response agyUsageResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return AntigravityUsage{}, err
	}

	usage := AntigravityUsage{}
	for _, group := range response.Command.Data.Groups {
		parsedGroup := AntigravityUsageGroup{Name: group.Name}
		for _, bucket := range group.Buckets {
			parsedGroup.Buckets = append(parsedGroup.Buckets, AntigravityUsageBucket{
				Name:      bucket.Name,
				Window:    bucket.Window,
				Remaining: bucket.RemainingFraction * 100,
				ResetTime: bucket.ResetTime,
			})
		}
		usage.Groups = append(usage.Groups, parsedGroup)
	}
	return usage, nil
}

// resolveAgyPath returns the agy executable to run: lookupPath as found by
// exec.LookPath("agy") when lookupErr is nil, otherwise the fallback install
// location under homeDir(), verified to exist via pathExists. Accepting
// homeDir/pathExists as functions (rather than calling os.UserHomeDir/os.Stat
// directly) keeps this pure and unit-testable without touching the real
// filesystem, and returning the resolved path as a single value (instead of
// reassigning an outer variable from a nested scope) rules out the kind of
// shadowing bug this used to have, where a `:=` inside the fallback branch
// silently left the caller's lookupPath untouched.
func resolveAgyPath(lookupPath string, lookupErr error, homeDir func() (string, error), pathExists func(string) bool) (string, error) {
	if lookupErr == nil {
		return lookupPath, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", errors.New("Checked PATH.")
	}
	fallbackPath, ok := antigravityFallbackPath(home)
	if !ok {
		return "", errors.New("Checked PATH.")
	}
	if !pathExists(fallbackPath) {
		return "", fmt.Errorf("Checked PATH and %s.", fallbackPath)
	}
	return fallbackPath, nil
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

// antigravityAuthMarkers are the phrases that let a failed agy command be read
// as "signed out" rather than "something went wrong". The list is deliberately
// short: an unrecognized failure must fall through to a temporary error, since
// telling a signed-in user they are signed out is the worse mistake and the
// action plan forbids inferring a sign-out from an unknown result.
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

// antigravityUnsupportedMarkers are how agy rejects a command or flag it does
// not have. `agy auth status` on 1.1.28 answers with the second of these.
var antigravityUnsupportedMarkers = []string{
	"unknown flag",
	"unexpected argument",
	"unknown command",
	"unknown subcommand",
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

func GetAntigravityUsage() AntigravityUsage {
	return getAntigravityUsage(context.Background(), defaultDeps(), true)
}

// findAgy resolves the executable. The raw lookup failure is kept under
// Details for transparency, while Message provides clear guidance.
func findAgy(deps providerDeps) (string, Diagnosis, bool) {
	lookupPath, lookupErr := deps.lookPath("agy")
	agyPath, err := resolveAgyPath(lookupPath, lookupErr, deps.homeDir, deps.pathExists)
	if err != nil {
		return "", Diagnosis{
			Status:  StatusNotInstalled,
			Message: "Install the Antigravity CLI (<code>agy</code>) and sign in to monitor your quota.",
			Details: technicalDetails(err.Error()),
		}, false
	}
	return agyPath, Diagnosis{}, true
}

// diagnoseAntigravityLocal answers with what can be known offline: whether agy
// is installed and whether its version is supported. It never runs `/usage`,
// which makes it the safe call for the onboarding screen.
func diagnoseAntigravityLocal(ctx context.Context, deps providerDeps) Diagnosis {
	agyPath, notInstalled, ok := findAgy(deps)
	if !ok {
		return notInstalled
	}
	diagnosis, _ := diagnoseAntigravity(ctx, deps.runner, agyPath, false)
	return diagnosis
}

func getAntigravityUsage(ctx context.Context, deps providerDeps, active bool) AntigravityUsage {
	usage := AntigravityUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	agyPath, notInstalled, found := findAgy(deps)
	if !found {
		usage.applyDiagnosis(notInstalled)
		return usage
	}

	diagnosis, ok := diagnoseAntigravity(ctx, deps.runner, agyPath, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	usageCtx, cancel := context.WithTimeout(ctx, antigravityUsageTimeout)
	defer cancel()
	result, runErr := deps.runner.run(usageCtx, agyPath,
		"-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	if runErr != nil {
		usage.applyDiagnosis(Diagnosis{
			Status:  StatusTemporaryError,
			Message: "Could not reach Antigravity right now. Retry in a moment.",
			Details: technicalDetails(runErr.Error() + " " + result.Stderr),
		})
		return usage
	}

	groups, diagnosis := classifyAntigravityUsage(ctx, deps, agyPath, result)
	if diagnosis.Status == StatusConnected {
		usage.Groups = groups
	}
	usage.applyDiagnosis(diagnosis)
	return usage
}

// classifyAntigravityUsage turns one `/usage` run into a state. `/usage` is the
// single path that proves sign-in and usage access at once, so a clean run with
// at least one group is the only thing that yields StatusConnected.
func classifyAntigravityUsage(ctx context.Context, deps providerDeps, agyPath string, result commandResult) ([]AntigravityUsageGroup, Diagnosis) {
	if result.ExitCode == 0 {
		parsed, err := parseAntigravityUsage([]byte(result.Stdout))
		if err == nil && len(parsed.Groups) > 0 {
			return parsed.Groups, Diagnosis{Status: StatusConnected}
		}
		// Exit 0 means agy authenticated and answered; we just cannot show it.
		reason := ReasonNoUsageData
		if err != nil {
			reason = ReasonUnsupportedResponse
		}
		return nil, usageUnreadableDiagnosis("Antigravity", reason, err)
	}

	output := result.Stdout + " " + result.Stderr
	if containsAnyMarker(output, antigravityAuthMarkers) {
		return nil, Diagnosis{
			Status:  StatusSignInRequired,
			Message: "Sign in to the Antigravity CLI to view quota information.",
			Details: technicalDetails(output),
		}
	}
	if containsAnyMarker(output, antigravityUnsupportedMarkers) {
		return nil, Diagnosis{
			Status:  StatusUnsupportedCLI,
			Message: "This Antigravity CLI version is not supported. Update the CLI.",
			Details: technicalDetails(output),
		}
	}
	return nil, classifyAntigravityWithModels(ctx, deps, agyPath, output)
}

// classifyAntigravityWithModels is the optional secondary diagnostic. It runs
// only for a `/usage` failure we could not read, and only to answer one
// question: was that an authentication problem, or a usage response this
// version cannot handle? A working `agy models` proves the session is fine and
// narrows the failure to the response itself.
func classifyAntigravityWithModels(ctx context.Context, deps providerDeps, agyPath, usageOutput string) Diagnosis {
	modelsCtx, cancel := context.WithTimeout(ctx, antigravityModelsTimeout)
	defer cancel()
	result, err := deps.runner.run(modelsCtx, agyPath, "models")
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
			Status:  StatusSignInRequired,
			Message: "Sign in to the Antigravity CLI to view quota information.",
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
