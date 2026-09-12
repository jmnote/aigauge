// Command raw captures an unconverted usage response for fixture development.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	versionPattern     = regexp.MustCompile(`\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)
	unsafeFilenameChar = regexp.MustCompile(`[^A-Za-z0-9.-]+`)
	httpClient         = &http.Client{Timeout: 15 * time.Second}
)

func main() {
	if len(os.Args) != 2 {
		fatalf("usage: go run ./hack/fixtures/raw <codex|claude|antigravity|all>")
	}

	providers := []string{os.Args[1]}
	if os.Args[1] == "all" {
		providers = []string{"codex", "claude", "antigravity"}
	}

	for _, provider := range providers {
		if err := capture(provider); err != nil {
			fatalf("%s: %v", provider, err)
		}
	}
}

func capture(provider string) error {
	var cliName string
	switch provider {
	case "codex", "claude":
		cliName = provider
	case "antigravity":
		cliName = "agy"
	default:
		return fmt.Errorf("unknown provider %q", provider)
	}

	version, err := cliVersion(cliName)
	if err != nil {
		return err
	}

	var raw []byte
	switch provider {
	case "codex":
		raw, err = fetchCodex()
	case "claude":
		raw, err = fetchClaude()
	case "antigravity":
		raw, err = commandOutput("agy", "-p", "/usage", "--output-format", "json")
	}
	if err != nil {
		return err
	}

	formatted, err := formatJSON(raw)
	if err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	var planType string
	if provider == "codex" {
		formatted, err = redactCodex(formatted)
		if err != nil {
			return err
		}
		planType, err = codexPlanType(formatted)
		if err != nil {
			return err
		}
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("could not locate output directory")
	}
	var filename string
	if provider == "codex" {
		filename = fmt.Sprintf("%s-%s_%s_%s.json", provider, version, planType, time.Now().Format("2006-01-02"))
	} else {
		filename = fmt.Sprintf("%s-%s_%s.json", provider, version, time.Now().Format("2006-01-02"))
	}
	outputPath := filepath.Join(filepath.Dir(sourceFile), filename)
	if err := os.WriteFile(outputPath, formatted, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}

	fmt.Println("Wrote", outputPath)
	return nil
}

func cliVersion(name string) (string, error) {
	output, err := commandOutput(name, "--version")
	if err != nil {
		return "", err
	}
	version := versionPattern.FindString(string(output))
	if version == "" {
		return "", fmt.Errorf("could not determine %s version from %q", name, strings.TrimSpace(string(output)))
	}
	return version, nil
}

func commandOutput(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	if exitError, ok := err.(*exec.ExitError); ok && len(exitError.Stderr) != 0 {
		return nil, fmt.Errorf("%s: %s", name, strings.TrimSpace(string(exitError.Stderr)))
	}
	return nil, fmt.Errorf("%s: %w", name, err)
}

func fetchCodex() ([]byte, error) {
	var credentials struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := readCredentials(filepath.Join(".codex", "auth.json"), &credentials); err != nil {
		return nil, err
	}
	if credentials.Tokens.AccessToken == "" {
		return nil, fmt.Errorf("Codex credentials contain no access token")
	}
	return fetch("https://chatgpt.com/backend-api/wham/usage", map[string]string{
		"Authorization": "Bearer " + credentials.Tokens.AccessToken,
	})
}

func fetchClaude() ([]byte, error) {
	var credentials struct {
		ClaudeAIOAuth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := readCredentials(filepath.Join(".claude", ".credentials.json"), &credentials); err != nil {
		return nil, err
	}
	if credentials.ClaudeAIOAuth.AccessToken == "" {
		return nil, fmt.Errorf("Claude credentials contain no access token")
	}
	return fetch("https://api.anthropic.com/api/oauth/usage", map[string]string{
		"Authorization":  "Bearer " + credentials.ClaudeAIOAuth.AccessToken,
		"anthropic-beta": "oauth-2025-04-20",
	})
}

func readCredentials(relativePath string, target any) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	path := filepath.Join(home, relativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func fetch(url string, headers map[string]string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage request failed: %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

func formatJSON(raw []byte) ([]byte, error) {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, bytes.TrimSpace(raw), "", "  "); err != nil {
		return nil, err
	}
	formatted.WriteByte('\n')
	return formatted.Bytes(), nil
}

func codexPlanType(data []byte) (string, error) {
	var response struct {
		PlanType string `json:"plan_type"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("parse plan_type: %w", err)
	}
	if response.PlanType == "" {
		return "unknown", nil
	}
	// plan_type is interpolated straight into the output filename below, so
	// strip anything that isn't a safe filename character (e.g. a path
	// separator) rather than trusting the API response as-is.
	sanitized := unsafeFilenameChar.ReplaceAllString(response.PlanType, "-")
	if sanitized == "" {
		return "unknown", nil
	}
	return sanitized, nil
}

func redactCodex(data []byte) ([]byte, error) {
	redactions := map[string]string{
		"user_id": "user_abc",
		"email":   "user@example.com",
	}
	for field, replacement := range redactions {
		pattern := regexp.MustCompile(`(?m)^(\s*"` + regexp.QuoteMeta(field) + `"\s*:\s*)"[^"]*"`)
		if !pattern.Match(data) {
			return nil, fmt.Errorf("Codex response contains no %q field to redact", field)
		}
		data = pattern.ReplaceAll(data, []byte(`${1}"`+replacement+`"`))
	}
	return data, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, "raw:", fmt.Sprintf(format, args...))
	os.Exit(1)
}
