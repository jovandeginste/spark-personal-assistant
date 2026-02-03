package simplemarkdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type addParams struct {
	Item string `json:"item" jsonschema:"The todo item to add"`
}

type updateParams struct {
	Index   int    `json:"index" jsonschema:"The index of the item to update (0-based)"`
	NewItem string `json:"new_item" jsonschema:"The new text for the item"`
}

type toggleParams struct {
	Index int  `json:"index" jsonschema:"The index of the item to toggle (0-based)"`
	Done  bool `json:"done" jsonschema:"Set to true to mark as done, false for todo"`
}

func Register(server *mcp.Server, config Config, logger *slog.Logger) error {
	logger = logger.With("module", "simplemarkdown")
	logger.Info("Registering MCP package")

	if config.Path == "" {
		logger.Warn("No path configured for simplemarkdown")
		return nil
	}

	registerFetchTool(server, config, logger)
	registerAddTool(server, config, logger)
	registerUpdateTool(server, config, logger)
	registerToggleTool(server, config, logger)

	return nil
}

func registerFetchTool(server *mcp.Server, config Config, logger *slog.Logger) {
	// Tool: Fetch todo list
	fetchTool := &mcp.Tool{
		Name:        "todo_fetch",
		Description: "Fetch the current todo list",
	}

	fetchHandler := func(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
		content, err := os.ReadFile(config.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{
							Text: "Todo list is empty.",
						},
					},
				}, nil, nil
			}

			logger.Error("Failed to read todo file", "error", err)

			return nil, nil, fmt.Errorf("failed to read todo file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(content),
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, fetchTool, fetchHandler)
}

func registerAddTool(server *mcp.Server, config Config, logger *slog.Logger) {
	// Tool: Add item
	addTool := &mcp.Tool{
		Name:        "todo_add",
		Description: "Add a new item to the todo list",
	}

	addHandler := func(ctx context.Context, request *mcp.CallToolRequest, params addParams) (*mcp.CallToolResult, any, error) {
		f, err := os.OpenFile(config.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			logger.Error("Failed to open todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to open todo file: %w", err)
		}
		defer f.Close()

		item := strings.TrimSpace(params.Item)
		if item == "" {
			return nil, nil, errors.New("item cannot be empty")
		}

		if _, err := fmt.Fprintf(f, "- [ ] %s\n", item); err != nil {
			logger.Error("Failed to write to todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to write to todo file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "Added item: " + item,
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, addTool, addHandler)
}

func registerUpdateTool(server *mcp.Server, config Config, logger *slog.Logger) {
	// Tool: Update item
	updateTool := &mcp.Tool{
		Name:        "todo_update",
		Description: "Update an existing todo item",
	}

	updateHandler := func(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
		lines, err := readLines(config.Path)
		if err != nil {
			logger.Error("Failed to read todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to read todo file: %w", err)
		}

		if params.Index < 0 || params.Index >= len(lines) {
			return nil, nil, errors.New("index out of range")
		}

		line := lines[params.Index]
		if !strings.HasPrefix(line, "- [ ] ") && !strings.HasPrefix(line, "- [x] ") {
			return nil, nil, fmt.Errorf("line at index %d is not a todo item", params.Index)
		}

		prefix := line[:6]
		lines[params.Index] = prefix + params.NewItem

		if err := writeLines(config.Path, lines); err != nil {
			logger.Error("Failed to write todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to write todo file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Updated item at index %d", params.Index),
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, updateTool, updateHandler)
}

func registerToggleTool(server *mcp.Server, config Config, logger *slog.Logger) {
	// Tool: Toggle item
	toggleTool := &mcp.Tool{
		Name:        "todo_toggle",
		Description: "Mark an item as done or todo",
	}

	toggleHandler := func(ctx context.Context, request *mcp.CallToolRequest, params toggleParams) (*mcp.CallToolResult, any, error) {
		lines, err := readLines(config.Path)
		if err != nil {
			logger.Error("Failed to read todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to read todo file: %w", err)
		}

		if params.Index < 0 || params.Index >= len(lines) {
			return nil, nil, errors.New("index out of range")
		}

		line := lines[params.Index]
		if !strings.HasPrefix(line, "- [ ] ") && !strings.HasPrefix(line, "- [x] ") {
			return nil, nil, fmt.Errorf("line at index %d is not a todo item", params.Index)
		}

		content := line[6:]
		if params.Done {
			lines[params.Index] = "- [x] " + content
		} else {
			lines[params.Index] = "- [ ] " + content
		}

		if err := writeLines(config.Path, lines); err != nil {
			logger.Error("Failed to write todo file", "error", err)
			return nil, nil, fmt.Errorf("failed to write todo file: %w", err)
		}

		state := "todo"
		if params.Done {
			state = "done"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Marked item at index %d as %s", params.Index, state),
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, toggleTool, toggleHandler)
}

func readLines(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

func writeLines(path string, lines []string) error {
	output := strings.Join(lines, "\n")
	if len(lines) > 0 {
		output += "\n"
	}

	return os.WriteFile(path, []byte(output), 0o600)
}
