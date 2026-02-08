package simplemarkdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path  string   `mapstructure:"path"`
	Users []string `mapstructure:"users"`
}

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "simplemarkdown")),
	}
	return module
}

type addParams struct {
	Item string `json:"item" jsonschema:"The todo item to add"`
	User string `json:"user" jsonschema:"The user to add the item for"`
}

type updateParams struct {
	Index   int    `json:"index" jsonschema:"The index of the item to update (0-based)"`
	NewItem string `json:"new_item" jsonschema:"The new text for the item"`
	User    string `json:"user" jsonschema:"The user to update the item for"`
}

type toggleParams struct {
	Index int    `json:"index" jsonschema:"The index of the item to toggle (0-based)"`
	Done  bool   `json:"done" jsonschema:"Set to true to mark as done, false for todo"`
	User  string `json:"user" jsonschema:"The user to toggle the item for"`
}

type listUsersParams struct {
	Force bool `json:"force,omitempty" jsonschema:"Force refresh of the user list (optional)"`
}

func isValidUser(user string, validUsers []string) bool {
	if len(validUsers) == 0 {
		return false // Require explicit user configuration
	}
	for _, u := range validUsers {
		if strings.EqualFold(u, user) {
			return true
		}
	}
	return false
}

func (m *Module) Register(server *mcp.Server) error {
	m.Logger().Info("Registering MCP package")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_fetch",
		Description: "Fetch the current todo list",
	}, m.handleFetchTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_add",
		Description: "Add a new item to the todo list",
	}, m.handleAddTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_update",
		Description: "Update an existing todo item",
	}, m.handleUpdateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_toggle",
		Description: "Mark an item as done or todo",
	}, m.handleToggleTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "todo_list_users",
		Description: "List all users who have access to the todo list",
	}, m.handleListUsersTool)

	return nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.Path == "" {
		return errors.New("simplemarkdown path is not configured")
	}
	return nil
}

func (m *Module) handleFetchTool(ctx context.Context, request *mcp.CallToolRequest, params struct {
	Force bool   `json:"force,omitempty" jsonschema:"Force refresh (optional)"`
	User  string `json:"user" jsonschema:"The user to fetch the todo list for"`
},
) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger()
	logger.Debug("Fetch todo list", "user", params.User, "force", params.Force)
	if params.User == "" {
		return nil, nil, errors.New("user is required")
	}
	if !isValidUser(params.User, config.Users) {
		return nil, nil, fmt.Errorf("invalid user: %s", params.User)
	}

	path := getTodoPath(config.Path, params.User)
	content, err := os.ReadFile(path)
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

func (m *Module) handleAddTool(ctx context.Context, request *mcp.CallToolRequest, params addParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger()
	logger.Debug("Add todo item", "user", params.User, "item", params.Item)
	if params.User == "" {
		return nil, nil, errors.New("user is required")
	}
	if !isValidUser(params.User, config.Users) {
		return nil, nil, fmt.Errorf("invalid user: %s", params.User)
	}

	path := getTodoPath(config.Path, params.User)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
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

func (m *Module) handleUpdateTool(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger()
	logger.Debug("Update todo item", "user", params.User, "index", params.Index, "new_item", params.NewItem)
	if params.User == "" {
		return nil, nil, errors.New("user is required")
	}
	if !isValidUser(params.User, config.Users) {
		return nil, nil, fmt.Errorf("invalid user: %s", params.User)
	}

	path := getTodoPath(config.Path, params.User)
	lines, err := readLines(path)
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

	if err := writeLines(path, lines); err != nil {
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

func (m *Module) handleToggleTool(ctx context.Context, request *mcp.CallToolRequest, params toggleParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger()
	logger.Debug("Toggle todo item", "user", params.User, "index", params.Index, "done", params.Done)
	if params.User == "" {
		return nil, nil, errors.New("user is required")
	}
	if !isValidUser(params.User, config.Users) {
		return nil, nil, fmt.Errorf("invalid user: %s", params.User)
	}

	path := getTodoPath(config.Path, params.User)
	lines, err := readLines(path)
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

	if err := writeLines(path, lines); err != nil {
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

func (m *Module) handleListUsersTool(ctx context.Context, request *mcp.CallToolRequest, params listUsersParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger()
	logger.Debug("List todo users", "force", params.Force)
	users := config.Users
	if len(users) == 0 {
		logger.Info("No users configured")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No users configured"}}}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Users: " + strings.Join(users, ", "),
			},
		},
	}, nil, nil
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

func getTodoPath(basePath, user string) string {
	if user == "" {
		return basePath
	}
	ext := filepath.Ext(basePath)
	name := strings.TrimSuffix(basePath, ext)
	return fmt.Sprintf("%s.%s%s", name, user, ext)
}
