package diary

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

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

type addEntryParams struct {
	Entry string `json:"entry" jsonschema:"The text to add to the diary"`
	Date  string `json:"date,omitempty" jsonschema:"The date for the entry (YYYY-MM-DD), defaults to today"`
	Time  string `json:"time,omitempty" jsonschema:"The time for the entry (HH:MM), defaults to current time"`
	User  string `json:"user" jsonschema:"The user to add the entry for"`
}

type readParams struct {
	User      string `json:"user" jsonschema:"The user to retrieve entries for"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Start date (YYYY-MM-DD), inclusive"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"End date (YYYY-MM-DD), inclusive"`
}

type deleteParams struct {
	User string `json:"user" jsonschema:"The user whose entry to delete"`
	Date string `json:"date" jsonschema:"The date of the entry to delete (YYYY-MM-DD)"`
}

type updateParams struct {
	User  string `json:"user" jsonschema:"The user whose entry to update"`
	Date  string `json:"date" jsonschema:"The date of the entry to update (YYYY-MM-DD)"`
	Entry string `json:"entry" jsonschema:"The new text for the diary entry"`
}

type listUsersParams struct {
	Force bool `json:"force,omitempty" jsonschema:"Force refresh of the user list (optional)"`
}

func register(server *mcp.Server, config Config, logger *slog.Logger) error {
	if config.Path == "" {
		return errors.New("diary path is not configured")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_diary_entry",
		Description: "Adds a new entry to the user's diary. Use this to remember things or log daily events.",
	}, handleAddEntry(config))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_diary_entry",
		Description: "Updates an existing diary entry. Overwrites the entire entry for that day.",
	}, handleUpdateEntry(config))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_diary_entry",
		Description: "Deletes a diary entry for a specific date.",
	}, handleDeleteEntry(config))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_diary_users",
		Description: "Lists all users who have a diary.",
	}, handleListUsers(config))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_diary",
		Description: "Reads diary entries for a user, optionally filtered by date range.",
	}, handleReadDiary(config, logger))

	return nil
}

func (m *Module) Register(server *mcp.Server) error {
	config := m.Config().(Config)
	return register(server, config, m.Logger())
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

func handleAddEntry(config Config) func(ctx context.Context, request *mcp.CallToolRequest, params addEntryParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest, params addEntryParams) (*mcp.CallToolResult, any, error) {
		if params.User == "" {
			return nil, nil, errors.New("user is required")
		}
		if !isValidUser(params.User, config.Users) {
			return nil, nil, fmt.Errorf("invalid user: %s", params.User)
		}

		if params.Date == "" {
			params.Date = time.Now().Format("2006-01-02")
		}
		if params.Time == "" {
			params.Time = time.Now().Format("15:04")
		}

		userDir := filepath.Join(config.Path, params.User)
		if err := os.MkdirAll(userDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create user directory: %w", err)
		}

		filename := filepath.Join(userDir, params.Date+".md")
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open diary file: %w", err)
		}
		defer f.Close()

		if _, err := fmt.Fprintf(f, "\n%s: %s\n", params.Time, params.Entry); err != nil {
			return nil, nil, fmt.Errorf("failed to write to diary file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Added entry to %s's diary for %s at %s", params.User, params.Date, params.Time),
				},
			},
		}, nil, nil
	}
}

func handleUpdateEntry(config Config) func(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
		if params.User == "" {
			return nil, nil, errors.New("user is required")
		}
		if !isValidUser(params.User, config.Users) {
			return nil, nil, fmt.Errorf("invalid user: %s", params.User)
		}
		if params.Date == "" {
			return nil, nil, errors.New("date is required for update")
		}

		userDir := filepath.Join(config.Path, params.User)
		filename := filepath.Join(userDir, params.Date+".md")

		// Check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("entry not found for %s on %s", params.User, params.Date)
		}

		// Overwrite file
		if err := os.WriteFile(filename, []byte(params.Entry+"\n"), 0o600); err != nil {
			return nil, nil, fmt.Errorf("failed to update diary file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Updated entry for %s on %s", params.User, params.Date),
				},
			},
		}, nil, nil
	}
}

func handleDeleteEntry(config Config) func(ctx context.Context, request *mcp.CallToolRequest, params deleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest, params deleteParams) (*mcp.CallToolResult, any, error) {
		if params.User == "" {
			return nil, nil, errors.New("user is required")
		}
		if !isValidUser(params.User, config.Users) {
			return nil, nil, fmt.Errorf("invalid user: %s", params.User)
		}
		if params.Date == "" {
			return nil, nil, errors.New("date is required for deletion")
		}

		userDir := filepath.Join(config.Path, params.User)
		filename := filepath.Join(userDir, params.Date+".md")

		if err := os.Remove(filename); err != nil {
			if os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("entry not found for %s on %s", params.User, params.Date)
			}
			return nil, nil, fmt.Errorf("failed to delete diary file: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Deleted entry for %s on %s", params.User, params.Date),
				},
			},
		}, nil, nil
	}
}

func handleListUsers(config Config) func(ctx context.Context, request *mcp.CallToolRequest, params listUsersParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest, params listUsersParams) (*mcp.CallToolResult, any, error) {
		users := config.Users
		// Strict mode: if no users configured, return empty list (or could error)
		if len(users) == 0 {
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
}

func handleReadDiary(config Config, logger *slog.Logger) func(ctx context.Context, request *mcp.CallToolRequest, params readParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, request *mcp.CallToolRequest, params readParams) (*mcp.CallToolResult, any, error) {
		if params.User == "" {
			return nil, nil, errors.New("user is required")
		}
		if !isValidUser(params.User, config.Users) {
			return nil, nil, fmt.Errorf("invalid user: %s", params.User)
		}

		userDir := filepath.Join(config.Path, params.User)
		entries, err := os.ReadDir(userDir)
		if err != nil {
			if os.IsNotExist(err) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No entries found for user"}}}, nil, nil
			}
			return nil, nil, fmt.Errorf("failed to read user directory: %w", err)
		}

		var results []string
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
				continue
			}
			dateStr := strings.TrimSuffix(entry.Name(), ".md")

			if params.StartDate != "" && dateStr < params.StartDate {
				continue
			}
			if params.EndDate != "" && dateStr > params.EndDate {
				continue
			}

			content, err := os.ReadFile(filepath.Join(userDir, entry.Name()))
			if err != nil {
				logger.Error("failed to read diary entry", "file", entry.Name(), "error", err)
				continue
			}
			results = append(results, fmt.Sprintf("## %s\n%s", dateStr, string(content)))
		}

		if len(results) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No entries found"}}}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: strings.Join(results, "\n\n"),
				},
			},
		}, nil, nil
	}
}
