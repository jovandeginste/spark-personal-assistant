package generic

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetResourceCooldowns() {
	failedResources.Lock()
	failedResources.until = make(map[string]time.Time)
	failedResources.Unlock()
}

func TestHTTPFailureStartsCooldown(t *testing.T) {
	resetResourceCooldowns()
	t.Cleanup(resetResourceCooldowns)

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := getBody(server.URL)
	require.Error(t, err)

	_, err = getBody(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed recently")
	assert.Equal(t, int32(1), calls.Load())
}

func TestSuccessfulHTTPRequestClearsCooldown(t *testing.T) {
	resetResourceCooldowns()
	t.Cleanup(resetResourceCooldowns)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	markFailure(server.URL)

	_, err := getBody(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed recently")

	clearFailure(server.URL)

	body, err := getBody(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.NoError(t, checkCooldown(server.URL))
}

func TestExpiredCooldownIsRemoved(t *testing.T) {
	resetResourceCooldowns()
	t.Cleanup(resetResourceCooldowns)

	uri := "http://example.com/expired"

	failedResources.Lock()
	failedResources.until[uri] = time.Now().Add(-time.Second)
	failedResources.Unlock()

	assert.NoError(t, checkCooldown(uri))

	failedResources.Lock()
	_, ok := failedResources.until[uri]
	failedResources.Unlock()

	assert.False(t, ok)
}
