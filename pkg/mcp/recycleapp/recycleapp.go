package recycleapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jovandeginste/recycleapp-ics/pkg/recycleapp"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Zipcode     int    `mapstructure:"zipcode"`
	Street      string `mapstructure:"street"`
	HouseNumber int    `mapstructure:"house_number"`
	Lang        string `mapstructure:"lang"`
}

type collectionsParams struct {
	Zipcode     int    `json:"zipcode" jsonschema:"Zip code (e.g. 1000)"`
	Street      string `json:"street" jsonschema:"Street name"`
	HouseNumber int    `json:"house_number" jsonschema:"House number (digits only)"`
	Lang        string `json:"lang,omitempty" jsonschema:"Language of collection names (e.g. nl, fr, en, de), defaults to nl"`
	sparkmcp.DateRangeParams
}

type Module struct {
	sparkmcp.BaseModule
	client *recycleapp.Client
}

func New(config Config, logger *slog.Logger) *Module {
	m := &Module{}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "recycleapp"))
	m.client = recycleapp.NewClient(nil)
	return m
}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()
	logger.Info("Registering MCP package")

	tool := &mcp.Tool{
		Name:        "get_recycle_collections",
		Description: "Get recycle app garbage collections for a specific date or date range",
	}

	mcp.AddTool(server, tool, m.handleGetCollections)

	return nil
}

func (m *Module) handleGetCollections(ctx context.Context, request *mcp.CallToolRequest, params collectionsParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "get_recycle_collections")
	config := m.Config().(Config)

	// Fallback to config values if params are empty
	zipcode := params.Zipcode
	if zipcode == 0 {
		zipcode = config.Zipcode
	}
	street := params.Street
	if street == "" {
		street = config.Street
	}
	houseNumber := params.HouseNumber
	if houseNumber == 0 {
		houseNumber = config.HouseNumber
	}
	lang := params.Lang
	if lang == "" {
		lang = config.Lang
	}
	if lang == "" {
		lang = "nl"
	}

	if zipcode == 0 {
		return nil, nil, errors.New("zipcode parameter is required and cannot be 0")
	}
	if street == "" {
		return nil, nil, errors.New("street parameter is required")
	}
	if houseNumber == 0 {
		return nil, nil, errors.New("house_number parameter is required and cannot be 0")
	}

	start, end, err := params.ParseDateRange()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse date range: %w", err)
	}

	fromDate := start.Format("2006-01-02")
	untilDate := end.Format("2006-01-02")

	logger.Debug("Fetching collections", "zipcode", zipcode, "street", street, "houseNumber", houseNumber, "fromDate", fromDate, "untilDate", untilDate)

	client := m.client
	if client == nil {
		client = recycleapp.NewClient(nil)
	}

	zipcodeID, err := client.GetZipcodeID(ctx, zipcode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get zipcode ID: %w", err)
	}

	streetID, err := client.GetStreetID(ctx, zipcodeID, street)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get street ID: %w", err)
	}

	result, err := client.GetCollections(ctx, recycleapp.CollectionsParams{
		ZipcodeID:   zipcodeID,
		StreetID:    streetID,
		HouseNumber: houseNumber,
		FromDate:    fromDate,
		UntilDate:   untilDate,
		Lang:        lang,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get collections: %w", err)
	}

	events := result.ToJSONEvents()
	if len(events) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "[]",
				},
			},
		}, nil, nil
	}

	jsonBytes, err := json.Marshal(events)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal events to JSON: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonBytes),
			},
		},
	}, nil, nil
}

func (m *Module) Enabled() error {
	return nil
}
