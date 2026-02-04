package diary

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type diaryParams struct {
	Entry string `json:"entry" jsonschema:"The text to add to the diary"`
	Date  string `json:"date,omitempty" jsonschema:"The date for the entry (YYYY-MM-DD), defaults to today"`
	User  string `json:"user" jsonschema:"The user to add the entry for"`
}

func Register(server *mcp.Server, config Config, logger *slog.Logger) error {
	tool := &mcp.Tool{
		Name:        "add_diary_entry",
		Description: "Adds a new entry to the user's diary. Use this to remember things or log daily events.",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params diaryParams) (*mcp.CallToolResult, any, error) {
		if config.Path == "" {
			return nil, nil, fmt.Errorf("diary path is not configured")
		}

		if params.User == "" {
			return nil, nil, fmt.Errorf("user is required")
		}

		if params.Date == "" {
			params.Date = time.Now().Format("2006-01-02")
		}

		filename := filepath.Join(config.Path, params.User+".md")

		// Create directory if it doesn't exist
		if err := os.MkdirAll(config.Path, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create diary directory: %w", err)
		}

		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open diary file: %w", err)
		}
		defer f.Close()

		entry := fmt.Sprintf("\n## %s\n\n%s\n", params.Date, params.Entry)
		if _, err := f.WriteString(entry); err != nil {
			return nil, nil, fmt.Errorf("failed to write to diary file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Added entry to %s's diary for %s", params.User, params.Date),
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, tool, handler)
	return nil
}
