package jsonreader

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Sources []Source `mapstructure:"sources"`
}

type Source struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type Event struct {
	Date        time.Time `json:"Date"`
	Summary     string    `json:"Summary"`
	Description string    `json:"Description,omitempty"`
	Source      string    `json:"-"` // Internal field to track source name
}

// Custom UnmarshalJSON to handle various date formats if needed,
// for now we stick to the requested format but wrapped in a helper.
// The user example was: "2026-01-08T00:00:00.000Z" (RFC3339)
// Standard json unmarshal into time.Time handles RFC3339.

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger),
	}
	module.SetLogger(logger.With("module", "jsonreader"))
	return module
}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()
	logger.Info("Registering MCP package")

	dateTool := &mcp.Tool{
		Name:        "json_events",
		Description: "Search for events in configured JSON sources by date or date range",
	}

	queryTool := &mcp.Tool{
		Name:        "json_events_search",
		Description: "Search for events in configured JSON sources by keyword (case insensitive)",
	}

	mcp.AddTool(server, dateTool, m.handleEventsByDate)
	mcp.AddTool(server, queryTool, m.handleEventsByQuery)

	return nil
}

type dateParams struct {
	sparkmcp.DateRangeParams
}

type queryParams struct {
	Query string `json:"query" jsonschema:"The keyword to search for in summary or description"`
}

func (m *Module) handleEventsByDate(ctx context.Context, request *mcp.CallToolRequest, params dateParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "eventsByDate")

	logger.Debug("Search json events by date", "params", params)

	events := m.loadAllEvents(config)

	filtered := m.filterEventsByDate(events, params)

	if len(filtered) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No events found matching the date criteria."},
			},
		}, nil, nil
	}

	return m.formatEvents(filtered)
}

func (m *Module) handleEventsByQuery(ctx context.Context, request *mcp.CallToolRequest, params queryParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "eventsByQuery")

	logger.Debug("Search json events by query", "query", params.Query)

	events := m.loadAllEvents(config)

	filtered := m.filterEventsByQuery(events, params.Query)

	if len(filtered) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "No events found matching the query."},
			},
		}, nil, nil
	}

	return m.formatEvents(filtered)
}

func (m *Module) loadAllEvents(config Config) []Event {
	var allEvents []Event
	for _, source := range config.Sources {
		data, err := generic.GetBody(source.URL)
		if err != nil {
			m.Logger().Warn("Failed to read source", "name", source.Name, "url", source.URL, "error", err)
			continue
		}

		var events []Event
		if err := json.Unmarshal(data, &events); err != nil {
			m.Logger().Warn("Failed to parse source json", "name", source.Name, "error", err)
			continue
		}

		// Tag with source name
		for i := range events {
			events[i].Source = source.Name
		}
		allEvents = append(allEvents, events...)
	}
	return allEvents
}

func (m *Module) filterEventsByDate(events []Event, params dateParams) []Event {
	var results []Event

	// Parse parameters
	startStr, endStr := params.GetDateRange()
	var startDate, endDate time.Time
	var err error
	const layout = "2006-01-02"

	if startStr != "" {
		startDate, err = time.Parse(layout, startStr)
		if err != nil {
			m.Logger().Warn("Invalid start date format", "date", startStr)
		}
	}
	if endStr != "" {
		endDate, err = time.Parse(layout, endStr)
		if err != nil {
			m.Logger().Warn("Invalid end date format", "date", endStr)
		}
	}

	for _, e := range events {
		// Event date truncated to day for comparison
		eDate := e.Date.Truncate(24 * time.Hour)

		if !startDate.IsZero() && eDate.Before(startDate) {
			continue
		}

		if !endDate.IsZero() && eDate.After(endDate) {
			continue
		}

		results = append(results, e)
	}

	return results
}

func (m *Module) filterEventsByQuery(events []Event, query string) []Event {
	var results []Event
	q := strings.ToLower(query)

	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Summary), q) ||
			strings.Contains(strings.ToLower(e.Description), q) {
			results = append(results, e)
		}
	}
	return results
}

func (m *Module) formatEvents(events []Event) (*mcp.CallToolResult, any, error) {
	// Re-marshal to JSON for the LLM
	j, err := json.Marshal(events)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(j)},
		},
	}, nil, nil
}
