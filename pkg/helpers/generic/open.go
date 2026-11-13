package generic

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	dialTimeout    = 3 * time.Second
	requestTimeout = 30 * time.Second
)

// httpClient bounds resource downloads: DNS lookup and connection setup take at
// most dialTimeout, while the whole request (including reading the body) is
// capped at requestTimeout.
var httpClient = newHTTPClient()

func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}
}

// ReadResource reads a resource from a given URI, supporting file and http/https schemes.
// It returns an io.ReadCloser and an error. The caller is responsible for closing the ReadCloser.
func ReadResource(uri string) (io.ReadCloser, error) {
	return ReadResourceWithHeaders(uri, nil)
}

// ReadResourceWithHeaders reads a resource from a given URI, supporting file and http/https schemes.
// It allows passing custom HTTP headers.
// It returns an io.ReadCloser and an error. The caller is responsible for closing the ReadCloser.
func ReadResourceWithHeaders(uri string, headers map[string]string) (io.ReadCloser, error) {
	if uri == "-" {
		return os.Stdin, nil
	}

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
		return readHTTP(u, headers)
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

func readHTTP(u *url.URL, headers map[string]string) (io.ReadCloser, error) {
	return readHTTPWithClient(u, headers, httpClient)
}

func readHTTPWithClient(u *url.URL, headers map[string]string, client *http.Client) (io.ReadCloser, error) {
	uri := u.String()

	if err := checkCooldown(uri); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Spark")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		markFailure(uri)

		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() // Close the body on non-OK status
		markFailure(uri)

		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	clearFailure(uri)

	return resp.Body, nil
}
