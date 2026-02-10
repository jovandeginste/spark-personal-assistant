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
	Query string `json:"query" jsonschema:"Name, email, or address of the contact to find (case-insensitive)"`
}

type birthdayParams struct {
	Date      string `json:"date,omitempty" jsonschema:"Specific date to check for birthdays (MM-DD)"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Start date for birthday range search (MM-DD)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"End date for birthday range search (MM-DD)"`
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
	logger := m.Logger()

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
	logger := m.Logger()

	logger.Debug("Get birthdays", "date", params.Date, "start_date", params.StartDate, "end_date", params.EndDate)
	contacts, err := m.loadContacts(config.Path)
	if err != nil {
		logger.Error("Failed to load contacts", "error", err)
		return nil, nil, fmt.Errorf("failed to load contacts: %w", err)
	}

	var results []Contact

	// If specific date provided
	switch {
	case params.Date != "":
		date, err := parseDate(params.Date)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid date format: %w", err)
		}
		results = findBirthdays(contacts, date, date)
	case params.StartDate != "" && params.EndDate != "":
		start, err := parseDate(params.StartDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_date format: %w", err)
		}
		end, err := parseDate(params.EndDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end_date format: %w", err)
		}
		results = findBirthdays(contacts, start, end)
	default:
		// Default to today if no params
		now := time.Now()
		date := Date{Month: int(now.Month()), Day: now.Day()}
		results = findBirthdays(contacts, date, date)
	}

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
