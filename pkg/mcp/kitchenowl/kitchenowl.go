package kitchenowl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	APIURL      string
	Token       string
	HouseholdID int
}

type PlannedMeal struct {
	CookingDateUnixMili int64   `json:"cooking_date,omitempty"`
	CookingDate         string  `json:"cooking_date_string,omitempty"`
	HouseholdID         int     `json:"household_id"`
	Recipe              *Recipe `json:"recipe,omitempty"`
	Yields              any     `json:"yields"`
}

type Recipe struct {
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

type kitchenOwlParams struct{}

func Register(server *mcp.Server, config Config, cache caching.Cache, logger *slog.Logger) error {
	logger = logger.With("module", "kitchenowl")
	logger.Info("Registering MCP package")

	tool := &mcp.Tool{
		Name:        "mealplan",
		Description: "Get planned meals",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
		result, err := getPlannedMeals(config, cache)
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

	mcp.AddTool(server, tool, handler)

	// Tool 2: Force update meals
	updateTool := &mcp.Tool{
		Name:        "mealplan_update",
		Description: "Force update planned meals from KitchenOwl API",
	}

	updateHandler := func(ctx context.Context, request *mcp.CallToolRequest, params kitchenOwlParams) (*mcp.CallToolResult, any, error) {
		// Use ForceUpdateFile logic here
		u := fmt.Sprintf("%s/household/%d/planner", config.APIURL, config.HouseholdID)

		_, err := cache.ForceUpdateFile(u, func() (io.ReadCloser, error) {
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
	mcp.AddTool(server, updateTool, updateHandler)

	return nil
}

func getPlannedMeals(config Config, cache caching.Cache) ([]byte, error) {
	u := fmt.Sprintf("%s/household/%d/planner", config.APIURL, config.HouseholdID)

	// Check cache first
	if file, ok := cache.GetFile(u); ok {
		return os.ReadFile(file)
	}

	// Fetch from API and cache
	cachedFile, err := cache.SetFile(u, func() (io.ReadCloser, error) {
		return generic.ReadResourceWithHeaders(u, map[string]string{
			"Authorization": config.Token,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching/caching meals: %w", err)
	}

	return os.ReadFile(cachedFile)
}

func (m *PlannedMeal) SetDate() {
	m.CookingDate = time.UnixMilli(m.CookingDateUnixMili).Format("2006-01-02")
	m.CookingDateUnixMili = 0
}
