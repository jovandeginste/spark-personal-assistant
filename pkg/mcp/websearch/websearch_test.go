package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchDuckDuckGo(t *testing.T) {
	originalURL := duckDuckGoURL
	defer func() { duckDuckGoURL = originalURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test query", r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(`<div class="web-result">
<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F">Example</a>
<a class="result__snippet">An example result</a>
</div>`))
	}))
	defer server.Close()

	duckDuckGoURL = server.URL

	result, err := searchDuckDuckGo(context.Background(), "test query", 5)
	require.NoError(t, err)
	assert.Contains(t, result, "Title: Example")
	assert.Contains(t, result, "Description: An example result")
	assert.Contains(t, result, "URL: https://example.com/")
}

func TestSearchCooldown(t *testing.T) {
	resetCooldown := func() {
		cooldownMu.Lock()
		cooldownUntil = time.Time{}
		cooldownMu.Unlock()
	}

	resetCooldown()
	t.Cleanup(resetCooldown)

	originalURL := duckDuckGoURL
	t.Cleanup(func() { duckDuckGoURL = originalURL })

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	duckDuckGoURL = server.URL

	_, err := searchDuckDuckGo(context.Background(), "query", 5)
	require.Error(t, err)

	_, err = searchDuckDuckGo(context.Background(), "query", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed recently")
	assert.Equal(t, int32(1), calls.Load())
}

func TestSearchDuckDuckGoEmpty(t *testing.T) {
	originalURL := duckDuckGoURL
	defer func() { duckDuckGoURL = originalURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>No results</body></html>`))
	}))
	defer server.Close()

	duckDuckGoURL = server.URL

	result, err := searchDuckDuckGo(context.Background(), "nothing", 5)
	require.NoError(t, err)
	assert.Equal(t, "No good DuckDuckGo Search Results was found", result)
}
