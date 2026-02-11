package vcf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type Module struct {
	sparkmcp.BaseModule
	Cache caching.Cache
}

func New(config Config, cache caching.Cache, logger *slog.Logger) *Module {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "vcf")),
		Cache:      cache,
	}
	return module
}

type contactParams struct {
	Query []string `json:"query" jsonschema:"Names, emails, or addresses of the contacts to find (case-insensitive)"`
}

type birthdayParams struct {
	sparkmcp.DateRangeParams
}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()
	logger.Info("Registering MCP package")

	config := m.Config().(Config)
	config.Path = filepath.Clean(config.Path)
	_, err := m.loadContacts(config.Path)
	if err != nil {
		logger.Error("Failed to load contacts", "error", err)
	}

	tool := &mcp.Tool{
		Name:        "get_contact",
		Description: "Find a contact by name, email, or address (case-insensitive)",
	}

	mcp.AddTool(server, tool, m.handleGetContact)

	birthdayTool := &mcp.Tool{
		Name:        "get_birthdays",
		Description: "Get birthdays from VCF contacts for a specific date or date range",
	}

	mcp.AddTool(server, birthdayTool, m.handleGetBirthdays)

	return nil
}

func (m *Module) handleGetContact(ctx context.Context, request *mcp.CallToolRequest, params contactParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	config.Path = filepath.Clean(config.Path)
	logger := m.Logger().With("handler", "getContact")

	logger.Debug("Get contact", "query", params.Query)
	contacts, err := m.loadContacts(config.Path)
	if err != nil {
		logger.Error("Failed to load contacts", "error", err)
		return nil, nil, fmt.Errorf("failed to load contacts: %w", err)
	}

	results := findContactByName(contacts, params.Query)

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No contacts found.",
				},
			},
		}, nil, nil
	}

	jsonResult, err := json.Marshal(results)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonResult),
			},
		},
	}, nil, nil
}

func (m *Module) handleGetBirthdays(ctx context.Context, request *mcp.CallToolRequest, params birthdayParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	config.Path = filepath.Clean(config.Path)
	logger := m.Logger().With("handler", "getBirthdays")

	startDate, endDate := params.GetDateRange()
	logger.Debug("Get birthdays", "start_date", startDate, "end_date", endDate)
	contacts, err := m.loadContacts(config.Path)
	if err != nil {
		logger.Error("Failed to load contacts", "error", err)
		return nil, nil, fmt.Errorf("failed to load contacts: %w", err)
	}

	var results []Contact

	// Use generic helper to determine query mode
	// But VCF logic uses Date{Month, Day} structs, so we need to parse.
	// We can reuse the existing `parseDate` which handles MM-DD.
	// `DateRangeParams` uses YYYY-MM-DD or MM-DD.
	// `params.GetDateRange()` handles precedence.

	// Helper to parse safely
	safeParse := func(d string) (Date, error) {
		if d == "" {
			return Date{}, nil
		}
		return parseDate(d)
	}

	// Logic adaptation:
	var start, end Date
	if startDate == "" && endDate == "" {
		now := time.Now()
		start = Date{Month: int(now.Month()), Day: now.Day()}
		end = start
	} else {
		var err error
		start, err = safeParse(startDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_date format: %w", err)
		}
		if endDate != "" {
			end, err = safeParse(endDate)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid end_date format: %w", err)
			}
		} else {
			end = start
		}
	}
	results = findBirthdays(contacts, start, end)

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No birthdays found.",
				},
			},
		}, nil, nil
	}

	jsonResult, err := json.Marshal(results)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonResult),
			},
		},
	}, nil, nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.Path == "" {
		return errors.New("vcf path is not configured")
	}
	config.Path = filepath.Clean(config.Path)
	return nil
}
