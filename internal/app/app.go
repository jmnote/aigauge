package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var AppVersion = "dev"

type App struct{}

func (a *App) GetVersion() string {
	return AppVersion
}

type CodexUsage struct {
	Plan       string  `json:"plan"`
	FiveHour   float64 `json:"fiveHour"`
	SevenDay   float64 `json:"sevenDay"`
	FiveHourIn int     `json:"fiveHourResetIn"`
	SevenDayIn int     `json:"sevenDayResetIn"`
	FetchedAt  string  `json:"fetchedAt"`
	Error      string  `json:"error,omitempty"`
}

type AntigravityUsage struct {
	Groups    []AntigravityUsageGroup `json:"groups"`
	FetchedAt string                  `json:"fetchedAt"`
	Error     string                  `json:"error,omitempty"`
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

type codexAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAfterSeconds int     `json:"reset_after_seconds"`
		} `json:"primary_window"`
		SecondaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAfterSeconds int     `json:"reset_after_seconds"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
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

const createNoWindow = 0x08000000

var httpClient = &http.Client{Timeout: 10 * time.Second}

func (a *App) GetCodexUsage() CodexUsage {
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

	request, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to create request: %v", err)
		return usage
	}
	request.Header.Set("Authorization", "Bearer "+auth.Tokens.AccessToken)
	response, err := httpClient.Do(request)
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to fetch Codex usage: %v", err)
		return usage
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if bodyStr != "" && len(bodyStr) < 150 {
			usage.Error = fmt.Sprintf("Codex request failed (HTTP %d): %s", response.StatusCode, bodyStr)
		} else {
			usage.Error = fmt.Sprintf("Codex usage request failed (HTTP %s)", response.Status)
		}
		return usage
	}

	var data codexUsageResponse
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		usage.Error = fmt.Sprintf("Unable to parse Codex usage response: %v", err)
		return usage
	}
	usage.Plan = data.PlanType
	usage.FiveHour = data.RateLimit.PrimaryWindow.UsedPercent
	usage.SevenDay = data.RateLimit.SecondaryWindow.UsedPercent
	usage.FiveHourIn = data.RateLimit.PrimaryWindow.ResetAfterSeconds
	usage.SevenDayIn = data.RateLimit.SecondaryWindow.ResetAfterSeconds
	return usage
}

func (a *App) GetAntigravityUsage() AntigravityUsage {
	usage := AntigravityUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	agyPath, err := exec.LookPath("agy")
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			usage.Error = "agy command not found"
			return usage
		}
		agyPath = filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe")
		if _, statErr := os.Stat(agyPath); statErr != nil {
			usage.Error = "agy command not found in PATH or AppData"
			return usage
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, agyPath, "-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			usage.Error = "agy usage request timed out (45s)"
		} else {
			errMsg := strings.TrimSpace(string(output))
			if errMsg != "" {
				usage.Error = fmt.Sprintf("agy failed: %s", errMsg)
			} else {
				usage.Error = fmt.Sprintf("Failed to fetch agy usage: %v", err)
			}
		}
		return usage
	}

	var response agyUsageResponse
	if err := json.Unmarshal(output, &response); err != nil {
		usage.Error = fmt.Sprintf("Unable to parse agy usage response: %v", err)
		return usage
	}

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
	if len(usage.Groups) == 0 {
		usage.Error = "No agy usage data found"
	}
	return usage
}
