package websearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultDuckDuckGoURL = "https://html.duckduckgo.com/html/"
	duckDuckGoUserAgent  = "Spark personal assistant"
	searchTimeout        = 30 * time.Second
	searchCooldown       = 5 * time.Minute
)

var duckDuckGoURL = defaultDuckDuckGoURL

var searchClient = &http.Client{Timeout: searchTimeout}

var (
	cooldownMu    sync.Mutex
	cooldownUntil time.Time
)

type searchResult struct {
	Title string
	Info  string
	Ref   string
}

func searchDuckDuckGo(ctx context.Context, query string, maxResults int) (string, error) {
	if err := checkSearchCooldown(); err != nil {
		return "", err
	}

	if maxResults <= 0 {
		maxResults = 10
	}

	success := false

	defer func() {
		if !success {
			markSearchFailure()
		}
	}()

	params := url.Values{}
	params.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, duckDuckGoURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create duckduckgo request: %w", err)
	}
	req.Header.Set("User-Agent", duckDuckGoUserAgent)

	resp, err := searchClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("duckduckgo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("duckduckgo returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read duckduckgo response: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to parse duckduckgo response: %w", err)
	}

	results := make([]searchResult, 0, maxResults)

	doc.Find(".web-result").EachWithBreak(func(_ int, result *goquery.Selection) bool {
		if len(results) >= maxResults {
			return false
		}

		titleNode := result.Find(".result__a")
		ref := titleNode.AttrOr("href", "")

		if parsed, err := url.Parse(ref); err == nil {
			if target := parsed.Query().Get("uddg"); target != "" {
				ref = target
			}
		}

		results = append(results, searchResult{
			Title: strings.TrimSpace(titleNode.Text()),
			Info:  strings.TrimSpace(result.Find(".result__snippet").Text()),
			Ref:   ref,
		})

		return true
	})

	success = true
	clearSearchCooldown()

	if len(results) == 0 {
		return "No good DuckDuckGo Search Results was found", nil
	}

	var output strings.Builder
	for _, result := range results {
		fmt.Fprintf(&output, "Title: %s\nDescription: %s\nURL: %s\n\n", result.Title, result.Info, result.Ref)
	}

	return output.String(), nil
}

func checkSearchCooldown() error {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()

	if cooldownUntil.IsZero() {
		return nil
	}

	if time.Now().After(cooldownUntil) {
		cooldownUntil = time.Time{}

		return nil
	}

	return fmt.Errorf("websearch failed recently, retry in %s", time.Until(cooldownUntil).Round(time.Second))
}

func markSearchFailure() {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()

	cooldownUntil = time.Now().Add(searchCooldown)
}

func clearSearchCooldown() {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()

	cooldownUntil = time.Time{}
}
