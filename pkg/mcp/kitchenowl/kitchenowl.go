package kitchenowl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	APIURL      string
	Token       string
	HouseholdID int
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

func (m *Module) Enabled() error {
	config := m.Config().(Config)
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

func (m *PlannedMeal) SetDate() {
	m.CookingDate = time.UnixMilli(m.CookingDateUnixMili).Format("2006-01-02")
	m.CookingDateUnixMili = 0
}
