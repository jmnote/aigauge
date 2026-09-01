package providers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// fetchAuthorizedJSON performs an authenticated GET request against url with
// the given headers and returns the raw response body. Network failures and
// non-200 responses are translated into an error prefixed with providerLabel
// so callers can surface it directly as a usage.Error string.
func fetchAuthorizedJSON(url, providerLabel string, headers map[string]string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s usage: %w", providerLabel, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if bodyStr != "" && len(bodyStr) < 150 {
			return nil, fmt.Errorf("%s usage request failed (HTTP %d): %s", providerLabel, response.StatusCode, bodyStr)
		}
		return nil, fmt.Errorf("%s usage request failed (HTTP %s)", providerLabel, response.Status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s usage response: %w", providerLabel, err)
	}
	return body, nil
}
