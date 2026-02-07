package googlecontacts

import (
	"context"
	"encoding/json"
	"errors"
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

type dateParams struct {
	Date      string `json:"date,omitempty" jsonschema:"The date to search for (MM-DD)"`
	StartDate string `json:"start_date,omitempty" jsonschema:"The start date to search for (MM-DD)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"The end date to search for (MM-DD)"`
}

type locationParams struct {
	Query string `json:"query" jsonschema:"The city, country, or other location keyword to search for"`
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

	dateTool := &mcp.Tool{
		Name:        "google_contacts_by_date",
		Description: "Search for Google Contacts by date (birthday, anniversary, etc)",
	}

	dateHandler := func(ctx context.Context, request *mcp.CallToolRequest, params dateParams) (*mcp.CallToolResult, any, error) {
		result, err := searchContactsByDate(ctx, config, params)
		if err != nil {
			logger.Error("Failed to search contacts by date", "params", params, "error", err)
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

	mcp.AddTool(server, dateTool, dateHandler)

	locationTool := &mcp.Tool{
		Name:        "google_contacts_by_location",
		Description: "Search for Google Contacts by location (address, city, country, etc)",
	}

	locationHandler := func(ctx context.Context, request *mcp.CallToolRequest, params locationParams) (*mcp.CallToolResult, any, error) {
		result, err := searchContactsByLocation(ctx, config, params)
		if err != nil {
			logger.Error("Failed to search contacts by location", "params", params, "error", err)
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

	mcp.AddTool(server, locationTool, locationHandler)

	return nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.TokenFile == "" {
		return errors.New("googlecontacts token file is not configured")
	}
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
	connections, err := fetchAllConnections(srv)
	if err != nil {
		return "", fmt.Errorf("unable to list connections: %w", err)
	}

	if len(connections) == 0 {
		return "No contacts found.", nil
	}

	j, err := json.Marshal(connections)
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func fetchAllConnections(srv *people.Service) ([]*people.Person, error) {
	var allConnections []*people.Person
	pageToken := ""

	for {
		call := srv.People.Connections.List("people/me").
			PersonFields(strings.Join(readMask, ",")).
			PageSize(100)

		if pageToken != "" {
			call.PageToken(pageToken)
		}

		r, err := call.Do()
		if err != nil {
			return nil, err
		}

		allConnections = append(allConnections, r.Connections...)

		pageToken = r.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return allConnections, nil
}

func searchContactsByDate(ctx context.Context, config Config, params dateParams) (string, error) {
	client := getClient(&config)

	srv, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("unable to retrieve people client: %w", err)
	}

	connections, err := fetchAllConnections(srv)
	if err != nil {
		return "", fmt.Errorf("unable to list connections: %w", err)
	}

	var results []*people.Person
	for _, person := range connections {
		if matchesDate(person, params) {
			results = append(results, person)
		}
	}

	if len(results) == 0 {
		return "No contacts found.", nil
	}

	j, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func matchesDate(person *people.Person, params dateParams) bool {
	for _, birthday := range person.Birthdays {
		if checkDate(birthday.Date, params) {
			return true
		}
	}
	for _, event := range person.Events {
		if checkDate(event.Date, params) {
			return true
		}
	}
	return false
}

func checkDate(date *people.Date, params dateParams) bool {
	if date == nil {
		return false
	}
	// Format as MM-DD
	d := fmt.Sprintf("%02d-%02d", date.Month, date.Day)

	if params.Date != "" && d == params.Date {
		return true
	}

	if params.StartDate != "" && params.EndDate != "" {
		if params.StartDate <= params.EndDate {
			// Normal range (e.g. 01-01 to 01-31)
			if d >= params.StartDate && d <= params.EndDate {
				return true
			}
		} else {
			// Wrap around range (e.g. 12-01 to 01-31)
			if d >= params.StartDate || d <= params.EndDate {
				return true
			}
		}
	}

	return false
}

func searchContactsByLocation(ctx context.Context, config Config, params locationParams) (string, error) {
	client := getClient(&config)

	srv, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("unable to retrieve people client: %w", err)
	}

	connections, err := fetchAllConnections(srv)
	if err != nil {
		return "", fmt.Errorf("unable to list connections: %w", err)
	}

	var results []*people.Person
	for _, person := range connections {
		if matchesLocation(person, params.Query) {
			results = append(results, person)
		}
	}

	if len(results) == 0 {
		return "No contacts found.", nil
	}

	j, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func matchesLocation(person *people.Person, query string) bool {
	q := strings.ToLower(query)
	for _, address := range person.Addresses {
		// Check formatted address
		if strings.Contains(strings.ToLower(address.FormattedValue), q) {
			return true
		}
		// Check components if available
		if strings.Contains(strings.ToLower(address.City), q) {
			return true
		}
		if strings.Contains(strings.ToLower(address.Country), q) {
			return true
		}
		if strings.Contains(strings.ToLower(address.Region), q) {
			return true
		}
		if strings.Contains(strings.ToLower(address.StreetAddress), q) {
			return true
		}
	}
	// Also check locations field if present (though often redundant with addresses)
	for _, loc := range person.Locations {
		if strings.Contains(strings.ToLower(loc.Value), q) {
			return true
		}
	}
	return false
}
