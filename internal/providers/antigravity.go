package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

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

// Workaround: agy usage can intermittently flash a PowerShell/console window on Windows.
// agy 1.1.22 expects a console/TTY; CREATE_NO_WINDOW can hang (see
// https://github.com/google-antigravity/antigravity-cli/issues/508), so use a
// hidden console and window instead.
const createNewConsole = 0x00000010

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

func GetAntigravityUsage() AntigravityUsage {
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
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewConsole,
		HideWindow:    true,
	}
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

	parsed, err := parseAntigravityUsage(output)
	if err != nil {
		usage.Error = fmt.Sprintf("Unable to parse agy usage response: %v", err)
		return usage
	}
	usage.Groups = parsed.Groups
	if len(usage.Groups) == 0 {
		usage.Error = "No agy usage data found"
	}
	return usage
}
