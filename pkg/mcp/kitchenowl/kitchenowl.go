package kitchenowl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	APIURL         string
	Token          string
	HouseholdID    int
	ShoppingListID int
}

type Module struct {
	sparkmcp.BaseModule
	Cache caching.Cache
}

func New(config Config, cache caching.Cache, logger *slog.Logger) *Module {
	m := &Module{Cache: cache}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "kitchenowl"))
	return m
}

type PlannedMeal struct {
	CookingDateUnixMili int64   `json:"cooking_date,omitempty"`
	CookingDate         string  `json:"cooking_date_string,omitempty"`
	HouseholdID         int     `json:"household_id"`
	Recipe              *Recipe `json:"recipe,omitempty"`
	Yields              any     `json:"yields"`
}

type Recipe struct {
	ID              int    `json:"id"`
	CookTime        int    `json:"cook_time"`
	Description     string `json:"description"`
	Name            string `json:"name"`
	PrepTime        int    `json:"prep_time"`
	Source          string `json:"source"`
	SuggestionRank  int    `json:"suggestion_rank"`
	SuggestionScore int    `json:"suggestion_score"`
	Tags            []any  `json:"tags,omitempty"`
	Time            int    `json:"time"`
	Yields          int    `json:"yields"`
}

type kitchenOwlParams struct {
	Force bool `json:"force,omitempty" jsonschema:"Force refresh of the data (optional)"`
}

type recipeParams struct {
	ID    int  `json:"id" jsonschema:"ID of the recipe to fetch"`
	Force bool `json:"force,omitempty" jsonschema:"Force refresh of the data (optional)"`
}

type recipeSearchParams struct {
	Query []string `json:"query" jsonschema:"Search query"`
}

type itemSearchParams struct {
	Query []string `json:"query" jsonschema:"Search query"`
}

type shoppingListAddParams struct {
	Name string `json:"name" jsonschema:"Name of the item to add"`
}

func (m *Module) Register(server *mcp.Server) error {
	m.Logger().Info("Registering MCP package")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealplan",
		Description: "Get planned meals",
	}, m.handleMealPlan)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealplan_update",
		Description: "Force update planned meals from KitchenOwl API",
	}, m.handleMealPlanUpdate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "recipes",
		Description: "Get all recipes",
	}, m.handleRecipes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "recipe",
		Description: "Get details of a single recipe",
	}, m.handleRecipe)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "recipe_search",
		Description: "Search for recipes by name or description",
	}, m.handleRecipeSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "shoppinglist_search_ingredients",
		Description: "Search for all available ingredients",
	}, m.handleIngredientsSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "shoppinglist_list_ingredients",
		Description: "Get shopping list items",
	}, m.handleShoppingList)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "shoppinglist_add_ingredient",
		Description: "Add an ingredient to the shopping list (first search for existing ingredients, and available ingredients, to prevent duplicates)",
	}, m.handleShoppingListAdd)

	return nil
}

func (m *Module) handleMealPlan(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "mealplan")
	logger.Debug("Fetching planned meals", "params", params)
	result, err := m.getPlannedMeals()
	if err != nil {
		logger.Error("Failed to get planned meals", "error", err)
		return nil, nil, err
	}

	var meals []PlannedMeal
	if err := json.Unmarshal(result, &meals); err != nil {
		logger.Error("Failed to unmarshal mcp result", "error", err)
		return nil, nil, fmt.Errorf("failed to unmarshal mcp result: %w", err)
	}

	for i := range meals {
		meals[i].SetDate()
	}

	mealsJSON, err := json.Marshal(meals)
	if err != nil {
		logger.Error("Failed to marshal mcp result", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal mcp result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Planned meals: %s", mealsJSON),
			},
		},
	}, nil, nil
}

func (m *Module) handleMealPlanUpdate(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "mealplanUpdate")
	logger.Debug("Forcing update of planned meals")
	// Use ForceUpdateFile logic here
	u := fmt.Sprintf("%s/household/%d/planner", config.APIURL, config.HouseholdID)

	_, err := m.Cache.ForceUpdateFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		logger.Error("Failed to update planned meals", "error", err)
		return nil, nil, fmt.Errorf("failed to update planned meals: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully updated planned meals cache.",
			},
		},
	}, nil, nil
}

func (m *Module) handleRecipes(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "recipes")
	logger.Debug("Fetching recipes", "params", params)

	var result []byte
	var err error

	if params.Force {
		result, err = m.forceGetRecipes()
	} else {
		result, err = m.getRecipes()
	}

	if err != nil {
		logger.Error("Failed to get recipes", "error", err)
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

func (m *Module) handleRecipe(ctx context.Context, request *mcp.CallToolRequest, params recipeParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "recipe")
	logger.Debug("Fetching recipe", "params", params)

	var result []byte
	var err error

	if params.Force {
		result, err = m.forceGetRecipe(params.ID)
	} else {
		result, err = m.getRecipe(params.ID)
	}

	if err != nil {
		logger.Error("Failed to get recipe", "error", err)
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

func (m *Module) handleRecipeSearch(ctx context.Context, request *mcp.CallToolRequest, params recipeSearchParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "recipeSearch")
	logger.Debug("Searching recipes", "params", params)

	result, err := m.getRecipes()
	if err != nil {
		logger.Error("Failed to get recipes", "error", err)
		return nil, nil, err
	}

	var recipes []Recipe
	if err := json.Unmarshal(result, &recipes); err != nil {
		logger.Error("Failed to unmarshal recipes", "error", err)
		return nil, nil, fmt.Errorf("failed to unmarshal recipes: %w", err)
	}

	var foundRecipes []Recipe
	for _, recipe := range recipes {
		for _, q := range params.Query {
			if strings.Contains(strings.ToLower(recipe.Name), strings.ToLower(q)) || strings.Contains(strings.ToLower(recipe.Description), strings.ToLower(q)) {
				foundRecipes = append(foundRecipes, recipe)
				break
			}
		}
	}

	foundRecipesJSON, err := json.Marshal(foundRecipes)
	if err != nil {
		logger.Error("Failed to marshal found recipes", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal found recipes: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(foundRecipesJSON),
			},
		},
	}, nil, nil
}

func (m *Module) handleIngredientsSearch(ctx context.Context, request *mcp.CallToolRequest, params itemSearchParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "itemSearch")
	logger.Debug("Searching items", "params", params)

	var allItems []map[string]any
	seenIDs := make(map[string]bool)

	for _, q := range params.Query {
		u := fmt.Sprintf("%s/household/%d/item/search?query=%s", config.APIURL, config.HouseholdID, url.QueryEscape(q))
		body, err := generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
		if err != nil {
			logger.Error("Failed to search items", "query", q, "error", err)
			continue
		}

		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			logger.Error("Failed to read response", "query", q, "error", err)
			continue
		}

		var items []map[string]any
		if err := json.Unmarshal(data, &items); err != nil {
			logger.Error("Failed to unmarshal response", "query", q, "error", err)
			continue
		}

		for _, item := range items {
			id := fmt.Sprintf("%v", item["id"])
			if seenIDs[id] {
				continue
			}
			seenIDs[id] = true
			allItems = append(allItems, item)
		}
	}

	resultJSON, err := json.Marshal(allItems)
	if err != nil {
		logger.Error("Failed to marshal results", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(resultJSON),
			},
		},
	}, nil, nil
}

func (m *Module) handleShoppingList(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "shoppinglist")
	logger.Debug("Fetching shopping list", "params", params)

	var result []byte
	var err error

	if params.Force {
		result, err = m.forceGetShoppingList()
	} else {
		result, err = m.getShoppingList()
	}

	if err != nil {
		logger.Error("Failed to get shopping list", "error", err)
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(result),
			},
		},
	}, nil, nil
}

func (m *Module) handleShoppingListAdd(ctx context.Context, request *mcp.CallToolRequest, params shoppingListAddParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "shoppinglistAdd")
	logger.Debug("Adding item to shopping list", "params", params)

	u := fmt.Sprintf("%s/shoppinglist/%d/add-item-by-name", config.APIURL, config.ShoppingListID)

	_, err := generic.PostJSON(u, map[string]string{
		"Authorization": config.Token,
	}, map[string]string{
		"name": params.Name,
	})
	if err != nil {
		logger.Error("Failed to add item to shopping list", "error", err)
		return nil, nil, err
	}

	// Invalidate cache for shopping list
	listURL := fmt.Sprintf("%s/shoppinglist/%d/items", config.APIURL, config.ShoppingListID)
	m.Cache.RemoveFile(listURL)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Added '%s' to the shopping list", params.Name),
			},
		},
	}, nil, nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.APIURL == "" {
		return errors.New("kitchenowl api url is not configured")
	}
	if config.Token == "" {
		return errors.New("kitchenowl token is not configured")
	}
	return nil
}

func (m *Module) getPlannedMeals() ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/household/%d/planner", config.APIURL, config.HouseholdID)

	// Check cache first
	if file, ok := m.Cache.GetFile(u); ok {
		return os.ReadFile(file)
	}

	// Fetch from API and cache
	cachedFile, err := m.Cache.SetFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching/caching meals: %w", err)
	}

	return os.ReadFile(cachedFile)
}

func (m *Module) getRecipes() ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/household/%d/recipe", config.APIURL, config.HouseholdID)

	// Check cache first
	if file, ok := m.Cache.GetFile(u); ok {
		return os.ReadFile(file)
	}

	return m.forceGetRecipes()
}

func (m *Module) forceGetRecipes() ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/household/%d/recipe", config.APIURL, config.HouseholdID)

	// Fetch from API and cache
	cachedFile, err := m.Cache.SetFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching/caching recipes: %w", err)
	}

	return os.ReadFile(cachedFile)
}

func (m *Module) getRecipe(id int) ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/recipe/%d", config.APIURL, id)

	// Check cache first
	if file, ok := m.Cache.GetFile(u); ok {
		return os.ReadFile(file)
	}

	return m.forceGetRecipe(id)
}

func (m *Module) forceGetRecipe(id int) ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/recipe/%d", config.APIURL, id)

	// Fetch from API and cache
	cachedFile, err := m.Cache.SetFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching/caching recipe: %w", err)
	}

	return os.ReadFile(cachedFile)
}

func (m *Module) getShoppingList() ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/shoppinglist/%d/items", config.APIURL, config.ShoppingListID)

	// Check cache first
	if file, ok := m.Cache.GetFile(u); ok {
		return os.ReadFile(file)
	}

	return m.forceGetShoppingList()
}

func (m *Module) forceGetShoppingList() ([]byte, error) {
	config := m.Config().(Config)
	u := fmt.Sprintf("%s/shoppinglist/%d/items", config.APIURL, config.ShoppingListID)

	// Fetch from API and cache
	cachedFile, err := m.Cache.SetFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching/caching shopping list: %w", err)
	}

	return os.ReadFile(cachedFile)
}

func (m *PlannedMeal) SetDate() {
	m.CookingDate = time.UnixMilli(m.CookingDateUnixMili).Format("2006-01-02")
	m.CookingDateUnixMili = 0
}
