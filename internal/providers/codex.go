package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CodexUsage struct {
	Plan       string  `json:"plan"`
	FiveHour   float64 `json:"fiveHour"`
	SevenDay   float64 `json:"sevenDay"`
	FiveHourIn int     `json:"fiveHourResetIn"`
	SevenDayIn int     `json:"sevenDayResetIn"`
	FetchedAt  string  `json:"fetchedAt"`
	Error      string  `json:"error,omitempty"`
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

func GetCodexUsage() CodexUsage {
	usage := CodexUsage{FetchedAt: time.Now().Format(time.RFC3339)}
	home, err := os.UserHomeDir()
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to get home directory: %v", err)
		return usage
	}

	authPath := filepath.Join(home, ".codex", "auth.json")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		usage.Error = "Codex login information not found (~/.codex/auth.json)"
		return usage
	}
	var auth codexAuth
	if err := json.Unmarshal(authData, &auth); err != nil || auth.Tokens.AccessToken == "" {
		usage.Error = "Unable to read Codex access token"
		return usage
	}

	body, err := fetchAuthorizedJSON("https://chatgpt.com/backend-api/wham/usage", "Codex", map[string]string{
		"Authorization": "Bearer " + auth.Tokens.AccessToken,
	})
	if err != nil {
		usage.Error = err.Error()
		return usage
	}

	parsed, err := parseCodexUsage(body)
	if err != nil {
		usage.Error = fmt.Sprintf("Unable to parse Codex usage response: %v", err)
		return usage
	}
	parsed.FetchedAt = usage.FetchedAt
	return parsed
}
