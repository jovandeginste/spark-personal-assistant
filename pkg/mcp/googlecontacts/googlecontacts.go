package googlecontacts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
)

type Config struct {
	Init      string `mapstructure:"init"`
	TokenFile string `mapstructure:"token_file"`
}

type Module struct {
	sparkmcp.BaseModule
}

type contactsParams struct {
	Query string `json:"query" jsonschema:"The query to search contacts for (name, email, phone)"`
}

var readMask = []string{
	"addresses", "ageRanges", "biographies", "birthdays", "calendarUrls", "clientData",
	"coverPhotos", "emailAddresses", "events", "externalIds", "genders", "imClients",
	"interests", "locales", "locations", "memberships", "metadata", "miscKeywords",
	"names", "nicknames", "occupations", "organizations", "phoneNumbers", "photos",
	"relations", "sipAddresses", "skills", "urls", "userDefined",
}

func (m *Module) Register(server *mcp.Server) error {
	config := m.Config().(Config)
	logger := m.Logger().With("module", "googlecontacts")
	logger.Info("Registering MCP package")

	tool := &mcp.Tool{
		Name:        "google_contacts",
		Description: "Search for Google Contacts",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params contactsParams) (*mcp.CallToolResult, any, error) {
		result, err := searchContacts(ctx, config, params.Query)
		if err != nil {
			logger.Error("Failed to search contacts", "query", params.Query, "error", err)
			return nil, nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: result,
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, tool, handler)

	return nil
}

func searchContacts(ctx context.Context, config Config, query string) (string, error) {
	client := getClient(&config)

	srv, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("unable to retrieve people client: %w", err)
	}

	// If query is empty, list connections instead of searching
	if query == "" {
		return listConnections(srv)
	}

	call := srv.People.SearchContacts().Query(query).ReadMask(strings.Join(readMask, ","))

	r, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("unable to search contacts: %w", err)
	}

	if len(r.Results) == 0 {
		return "No contacts found.", nil
	}

	j, err := json.Marshal(r.Results)
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func listConnections(srv *people.Service) (string, error) {
	call := srv.People.Connections.List("people/me").PersonFields(strings.Join(readMask, ",")).PageSize(100)

	r, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("unable to list connections: %w", err)
	}

	if len(r.Connections) == 0 {
		return "No contacts found.", nil
	}

	j, err := json.Marshal(r.Connections)
	if err != nil {
		return "", err
	}

	return string(j), nil
}
