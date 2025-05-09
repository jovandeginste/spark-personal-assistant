package generic

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ReadResource reads a resource from a given URI, supporting file and http/https schemes.
// It returns an io.ReadCloser and an error. The caller is responsible for closing the ReadCloser.
func ReadResource(uri string) (io.ReadCloser, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	switch strings.ToLower(u.Scheme) {
	case "", "file":
		// Handle local file paths
		return readFile(u)
	case "http", "https":
		// Handle HTTP/HTTPS URLs
		return readHTTP(u)
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}
}

func readFile(u *url.URL) (io.ReadCloser, error) {
	filePath := u.Path
	// On Windows, file URIs might start with a leading slash that needs to be removed
	// if the path includes a drive letter (e.g., file:///C:/path/to/file).
	if strings.HasPrefix(filePath, "/") && len(filePath) > 1 && (filePath[1] == ':' || strings.HasPrefix(filePath, "//")) {
		filePath = filePath[1:]
	}

	return os.Open(filePath)
}

func readHTTP(u *url.URL) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Spark")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() // Close the body on non-OK status
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	return resp.Body, nil
}
