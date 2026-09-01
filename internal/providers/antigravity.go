package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
		return "", errors.New("agy command not found")
	}
	fallbackPath, ok := antigravityFallbackPath(home)
	if !ok {
		return "", errors.New("agy command not found in PATH")
	}
	if !pathExists(fallbackPath) {
		return "", errors.New("agy command not found in PATH or the default installation directory")
	}
	return fallbackPath, nil
}

func GetAntigravityUsage() AntigravityUsage {
	usage := AntigravityUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	lookupPath, lookupErr := exec.LookPath("agy")
	agyPath, err := resolveAgyPath(lookupPath, lookupErr, os.UserHomeDir, func(path string) bool {
		_, statErr := os.Stat(path)
		return statErr == nil
	})
	if err != nil {
		usage.Error = err.Error()
		return usage
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, agyPath, "-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	configureAntigravityCommand(command)
	output, err := command.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			usage.Error = "agy usage request timed out (45s)"
		} else {
			errMsg := ""
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				errMsg = strings.TrimSpace(string(exitError.Stderr))
			}
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
