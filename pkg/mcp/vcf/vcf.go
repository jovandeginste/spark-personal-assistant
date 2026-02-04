package vcf

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type contactParams struct {
	Name string `json:"name" jsonschema:"Name of the contact to find"`
}

type birthdayParams struct {
	Date      string `json:"date,omitempty" jsonschema:"Specific date to check for birthdays (MM-DD)"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Start date for birthday range search (MM-DD)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"End date for birthday range search (MM-DD)"`
}

func Register(server *mcp.Server, config Config, cache caching.Cache, logger *slog.Logger) error {
	logger = logger.With("module", "vcf")
	logger.Info("Registering VCF package")

	if config.Path == "" {
		logger.Warn("No VCF path configured")
		return nil
	}

	registerBirthdayTool(server, config, cache, logger)
	registerContactTool(server, config, cache, logger)

	return nil
}

func registerContactTool(server *mcp.Server, config Config, cache caching.Cache, logger *slog.Logger) {
	tool := &mcp.Tool{
		Name:        "get_contact",
		Description: "Find a contact by name (case-insensitive)",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params contactParams) (*mcp.CallToolResult, any, error) {
		contacts, err := loadContacts(config.Path, cache)
		if err != nil {
			logger.Error("Failed to load contacts", "error", err)
			return nil, nil, fmt.Errorf("failed to load contacts: %w", err)
		}

		results := findContactByName(contacts, params.Name)

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

	mcp.AddTool(server, tool, handler)
}

func registerBirthdayTool(server *mcp.Server, config Config, cache caching.Cache, logger *slog.Logger) {
	tool := &mcp.Tool{
		Name:        "get_birthdays",
		Description: "Get birthdays from VCF contacts for a specific date or date range",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params birthdayParams) (*mcp.CallToolResult, any, error) {
		contacts, err := loadContacts(config.Path, cache)
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

	mcp.AddTool(server, tool, handler)
}
