package generic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PostJSON sends a POST request with a JSON body to the specified URL.
// It returns the response body as a byte slice.
func PostJSON(url string, headers map[string]string, body any) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Spark")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
