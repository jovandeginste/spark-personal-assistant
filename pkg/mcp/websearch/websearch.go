package websearch

import (
	"context"
	"fmt"
	"log/slog"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/robermar23/langchaingo/tools/duckduckgo"
)

type Config struct{}

type webSearchParams struct {
	Query      string `json:"query" jsonschema:"The search query"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"The maximum number of results to return (default 10)"`
}

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	m := &Module{}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "websearch"))
	return m
}

func (m *Module) Register(server *mcp.Server) error {
	tool := &mcp.Tool{
		Name:        "websearch",
		Description: "Search the web using DuckDuckGo",
	}

	mcp.AddTool(server, tool, m.handler)

	return nil
}

func (m *Module) handler(ctx context.Context, request *mcp.CallToolRequest, params webSearchParams) (*mcp.CallToolResult, any, error) {
	reqLogger := m.Logger()
	reqLogger.Debug("Searching the web", "query", params.Query, "max_results", params.MaxResults)

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	results, err := m.search(params.Query, maxResults)
	if err != nil {
		reqLogger.Error("Failed to search the web", "query", params.Query, "error", err)
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: results,
			},
		},
	}, nil, nil
}

func (m *Module) Enabled() error {
	return nil
}

func (m *Module) search(query string, maxResults int) (string, error) {
	tool, err := duckduckgo.New(maxResults, duckduckgo.DefaultUserAgent)
	if err != nil {
		return "", fmt.Errorf("failed to initialize duckduckgo search: %w", err)
	}

	result, err := tool.Call(context.Background(), query)
	if err != nil {
		return "", fmt.Errorf("failed to execute search: %w", err)
	}

	return result, nil
}
