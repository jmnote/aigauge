package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type CodexUsage struct {
	Plan       string  `json:"plan"`
	FiveHour   float64 `json:"fiveHour"`
	SevenDay   float64 `json:"sevenDay"`
	FiveHourIn int     `json:"fiveHourResetIn"`
	SevenDayIn int     `json:"sevenDayResetIn"`
	FetchedAt  string  `json:"fetchedAt"`

	// See DiagnosisFields in status.go: it carries the structured diagnosis,
	// shared by all three providers so applyDiagnosis is defined exactly once.
	DiagnosisFields
}

type codexAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow struct {
			UsedPercent       *float64 `json:"used_percent"`
			ResetAfterSeconds *int     `json:"reset_after_seconds"`
		} `json:"primary_window"`
		SecondaryWindow struct {
			UsedPercent       *float64 `json:"used_percent"`
			ResetAfterSeconds *int     `json:"reset_after_seconds"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
}

func parseCodexUsage(data []byte) (CodexUsage, error) {
	var response codexUsageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return CodexUsage{}, err
	}
	if response.RateLimit.PrimaryWindow.UsedPercent == nil ||
		response.RateLimit.PrimaryWindow.ResetAfterSeconds == nil ||
		response.RateLimit.SecondaryWindow.UsedPercent == nil ||
		response.RateLimit.SecondaryWindow.ResetAfterSeconds == nil {
		return CodexUsage{}, fmt.Errorf("response is missing required usage fields")
	}
	primaryUsed := *response.RateLimit.PrimaryWindow.UsedPercent
	secondaryUsed := *response.RateLimit.SecondaryWindow.UsedPercent
	primaryReset := *response.RateLimit.PrimaryWindow.ResetAfterSeconds
	secondaryReset := *response.RateLimit.SecondaryWindow.ResetAfterSeconds
	if primaryUsed < 0 || primaryUsed > 100 || secondaryUsed < 0 || secondaryUsed > 100 ||
		primaryReset < 0 || secondaryReset < 0 {
		return CodexUsage{}, fmt.Errorf("response contains out-of-range usage fields")
	}
	return CodexUsage{
		Plan:       response.PlanType,
		FiveHour:   primaryUsed,
		SevenDay:   secondaryUsed,
		FiveHourIn: primaryReset,
		SevenDayIn: secondaryReset,
	}, nil
}

// findCodexCredentials reads only the access token the usage request needs.
// See findCredentials in diagnose.go for what "found" means.
func findCodexCredentials(homeDir func() (string, error), readFile func(string) ([]byte, error)) (codexAuth, bool) {
	return findCredentials(homeDir, readFile, codexCredentialRelPath, func(a codexAuth) bool {
		return a.Tokens.AccessToken != ""
	})
}

func GetCodexUsage() CodexUsage {
	return getCodexUsage(context.Background(), defaultDeps(), true)
}

func getCodexUsage(ctx context.Context, deps providerDeps, active bool) CodexUsage {
	usage := CodexUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	diagnosis, credentials, ok := diagnoseCodex(ctx, deps, active)
	if !ok {
		usage.applyDiagnosis(diagnosis)
		return usage
	}

	body, err := fetchAuthorizedJSON("https://chatgpt.com/backend-api/wham/usage", "Codex", map[string]string{
		"Authorization": "Bearer " + credentials.Tokens.AccessToken,
	})
	if err != nil {
		usage.applyDiagnosis(usageFailureDiagnosis("Codex", err))
		return usage
	}

	parsed, err := parseCodexUsage(body)
	if err != nil {
		usage.applyDiagnosis(usageUnreadableDiagnosis("Codex", ReasonUnsupportedResponse, err))
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	parsed.Status = StatusConnected
	return parsed
}
