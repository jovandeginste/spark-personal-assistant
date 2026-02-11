package kitchenowl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		APIURL:         "http://example.com",
		Token:          "token",
		HouseholdID:    1,
		ShoppingListID: 1,
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
	module := New(config, cache, slog.Default())
	result, err := module.getPlannedMeals()
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
	module := New(config, cache, slog.Default())

	// 1. Initial Fetch
	_, err := module.getPlannedMeals()
	assert.NoError(t, err)
	assert.Equal(t, 1, requestCount)

	// 2. Fetch again immediately (should be cached)
	_, err = module.getPlannedMeals()
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
	_, err = module.getPlannedMeals()
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

func TestSearchRecipes(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []Recipe{
			{
				ID:          1,
				Name:        "Spaghetti Bolognese",
				Description: "Delicious pasta with meat sauce",
			},
			{
				ID:          2,
				Name:        "Vegetable Stir Fry",
				Description: "Healthy veggies with rice",
			},
			{
				ID:          3,
				Name:        "Chocolate Cake",
				Description: "Rich chocolate dessert",
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

	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := New(config, cache, logger)

	// Test case 1: Single query matching name
	params := recipeSearchParams{
		Query: []string{"Spaghetti"},
	}
	result, _, err := module.handleRecipeSearch(context.Background(), nil, params)
	assert.NoError(t, err)

	var foundRecipes []Recipe
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &foundRecipes)
	assert.NoError(t, err)
	assert.Len(t, foundRecipes, 1)
	assert.Equal(t, "Spaghetti Bolognese", foundRecipes[0].Name)

	// Test case 2: Multiple queries matching description and name
	params = recipeSearchParams{
		Query: []string{"healthy", "cake"},
	}
	result, _, err = module.handleRecipeSearch(context.Background(), nil, params)
	assert.NoError(t, err)

	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &foundRecipes)
	assert.NoError(t, err)
	assert.Len(t, foundRecipes, 2)
	// Order depends on how they are found, but should contain both
	names := []string{foundRecipes[0].Name, foundRecipes[1].Name}
	assert.Contains(t, names, "Vegetable Stir Fry")
	assert.Contains(t, names, "Chocolate Cake")

	// Test case 3: No match
	params = recipeSearchParams{
		Query: []string{"pizza"},
	}
	result, _, err = module.handleRecipeSearch(context.Background(), nil, params)
	assert.NoError(t, err)

	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &foundRecipes)
	assert.NoError(t, err)
	assert.Len(t, foundRecipes, 0)
}

func TestSearchItems(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/household/1/item/search", r.URL.Path)
		query := r.URL.Query().Get("query")

		var response []map[string]any
		switch query {
		case "milk":
			response = []map[string]any{
				{"id": 1, "name": "Whole Milk"},
				{"id": 2, "name": "Almond Milk"},
			}
		case "bread":
			response = []map[string]any{
				{"id": 3, "name": "Whole Wheat Bread"},
			}
		default:
			response = []map[string]any{}
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

	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := New(config, cache, logger)

	// Test case: Multiple queries
	params := itemSearchParams{
		Query: []string{"milk", "bread"},
	}
	result, _, err := module.handleItemSearch(context.Background(), nil, params)
	assert.NoError(t, err)

	var foundItems []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &foundItems)
	assert.NoError(t, err)

	// Should have 3 items total (2 milks + 1 bread)
	assert.Len(t, foundItems, 3)

	names := make([]string, 0, 3)
	for _, item := range foundItems {
		names = append(names, item["name"].(string))
	}
	assert.Contains(t, names, "Whole Milk")
	assert.Contains(t, names, "Almond Milk")
	assert.Contains(t, names, "Whole Wheat Bread")
}

func TestGetShoppingList(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/shoppinglist/1/items", r.URL.Path)
		assert.Equal(t, "token", r.Header.Get("Authorization"))

		response := []map[string]any{
			{
				"id":   1,
				"name": "Milk",
			},
			{
				"id":   2,
				"name": "Bread",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	config := Config{
		APIURL:         ts.URL,
		Token:          "token",
		HouseholdID:    1,
		ShoppingListID: 1,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := New(config, cache, logger)

	// Call handler
	params := kitchenOwlParams{}
	result, _, err := module.handleShoppingList(context.Background(), nil, params)
	assert.NoError(t, err)

	// Verify content
	var items []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &items)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Milk", items[0]["name"])
	assert.Equal(t, "Bread", items[1]["name"])
}

func TestAddShoppingListItem(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/shoppinglist/1/add-item-by-name", r.URL.Path)
		assert.Equal(t, "token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodPost, r.Method)

		var reqBody map[string]string
		bodyBytes, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(bodyBytes, &reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "Apples", reqBody["name"])

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer ts.Close()

	config := Config{
		APIURL:         ts.URL,
		Token:          "token",
		HouseholdID:    1,
		ShoppingListID: 1,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cache, _ := caching.NewService(os.TempDir(), time.Hour)
	module := New(config, cache, logger)

	// Call handler
	params := shoppingListAddParams{Name: "Apples"}
	result, _, err := module.handleShoppingListAdd(context.Background(), nil, params)
	assert.NoError(t, err)

	// Verify result
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "Added 'Apples' to the shopping list")
}
