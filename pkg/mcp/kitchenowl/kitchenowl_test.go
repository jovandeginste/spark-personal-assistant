package kitchenowl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, &mcp.ServerOptions{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		APIURL:      "http://example.com",
		Token:       "token",
		HouseholdID: 1,
	}

	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := &Module{Cache: cache}
	module.SetConfig(config)
	module.SetLogger(logger)
	err := module.Register(server)
	assert.NoError(t, err)
}

func TestGetPlannedMeals(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/household/1/planner", r.URL.Path)
		assert.Equal(t, "token", r.Header.Get("Authorization"))

		response := []PlannedMeal{
			{
				CookingDateUnixMili: time.Now().UnixMilli(),
				HouseholdID:         1,
				Recipe: &Recipe{
					Name: "Test Recipe",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	config := Config{
		APIURL:      ts.URL,
		Token:       "token",
		HouseholdID: 1,
	}

	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	result, err := getPlannedMeals(config, cache)
	assert.NoError(t, err)

	var meals []PlannedMeal

	err = json.Unmarshal(result, &meals)
	assert.NoError(t, err)
	assert.Len(t, meals, 1)
	assert.Equal(t, "Test Recipe", meals[0].Recipe.Name)
}

func TestCacheExpiry(t *testing.T) {
	// Mock server
	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		response := []PlannedMeal{{Recipe: &Recipe{Name: "Cached Recipe"}}}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	config := Config{
		APIURL:      ts.URL,
		Token:       "token",
		HouseholdID: 1,
	}

	tmpDir, _ := os.MkdirTemp("", "mcp-test-*")
	defer os.RemoveAll(tmpDir)

	cache, _ := caching.NewService(tmpDir, 12*time.Hour)

	// 1. Initial Fetch
	_, err := getPlannedMeals(config, cache)
	assert.NoError(t, err)
	assert.Equal(t, 1, requestCount)

	// 2. Fetch again immediately (should be cached)
	_, err = getPlannedMeals(config, cache)
	assert.NoError(t, err)
	assert.Equal(t, 1, requestCount)

	// 3. To test expiry, we need to inspect the cache internal file or manually
	// manipulate it. The caching service abstracts this away.
	// We can try to manually expire the file if we know the path.
	// But simpler: create a new service with 0 TTL for the next call?
	// Or trust the caching unit tests cover expiry.
	// Let's modify the file timestamp manually to be sure integration works.
	// The key is the URL.
	url := fmt.Sprintf("%s/household/%d/planner", config.APIURL, config.HouseholdID)

	file, ok := cache.GetFile(url)
	assert.True(t, ok, "File should be in cache")

	// Modify file time to simulate expiry (> 12 hours)
	oldTime := time.Now().Add(-13 * time.Hour)
	err = os.Chtimes(file, oldTime, oldTime)
	assert.NoError(t, err)

	// 4. Fetch again (should trigger new request because file is old)
	_, err = getPlannedMeals(config, cache)
	assert.NoError(t, err)
	assert.Equal(t, 2, requestCount)
}

func TestHandlerLogic(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []PlannedMeal{
			{
				CookingDateUnixMili: 1704067200000, // 2024-01-01
				HouseholdID:         1,
				Recipe: &Recipe{
					Name: "Test Recipe",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	config := Config{
		APIURL:      ts.URL,
		Token:       "token",
		HouseholdID: 1,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, &mcp.ServerOptions{})

	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := &Module{Cache: cache}
	module.SetConfig(config)
	module.SetLogger(logger)
	err := module.Register(server)
	assert.NoError(t, err)
}
