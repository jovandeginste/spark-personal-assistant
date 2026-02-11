package googlecontacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

func New(config Config, logger *slog.Logger) *Module {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger),
	}
	module.SetLogger(logger.With("module", "googlecontacts"))
	return module
}

type contactsParams struct {
	Query []string `json:"query" jsonschema:"The query to search contacts for (name, email, phone)"`
}

type dateParams struct {
	sparkmcp.DateRangeParams
}

type locationParams struct {
	Query []string `json:"query" jsonschema:"The city, country, or other location keyword to search for"`
}

var readMask = []string{
	"addresses", "ageRanges", "biographies", "birthdays", "calendarUrls", "clientData",
	"coverPhotos", "emailAddresses", "events", "externalIds", "genders", "imClients",
	"interests", "locales", "locations", "memberships", "metadata", "miscKeywords",
	"names", "nicknames", "occupations", "organizations", "phoneNumbers", "photos",
	"relations", "sipAddresses", "skills", "urls", "userDefined",
}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()
	logger.Info("Registering MCP package")

	tool := &mcp.Tool{
		Name:        "contacts",
		Description: "Search for Google Contacts",
	}

	mcp.AddTool(server, tool, m.handleContacts)

	dateTool := &mcp.Tool{
		Name:        "birthdays",
		Description: "Search for Google Contacts events by date (birthday, anniversary, etc)",
	}

	mcp.AddTool(server, dateTool, m.handleContactsByDate)

	locationTool := &mcp.Tool{
		Name:        "contacts_by_location",
		Description: "Search for Google Contacts by location (address, city, country, etc)",
	}

	mcp.AddTool(server, locationTool, m.handleContactsByLocation)

	return nil
}

func (m *Module) handleContacts(ctx context.Context, request *mcp.CallToolRequest, params contactsParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "contacts")

	logger.Debug("Search google contacts", "query", params.Query)
	result, err := m.searchContacts(ctx, config, params.Query)
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

func (m *Module) handleContactsByDate(ctx context.Context, request *mcp.CallToolRequest, params dateParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "contactsByDate")

	start, end := params.GetDateRange()
	logger.Debug("Search google contacts by date", "start", start, "end", end)
	result, err := m.searchContactsByDate(ctx, config, start, end)
	if err != nil {
		logger.Error("Failed to search contacts by date", "start", start, "end", end, "error", err)
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

func (m *Module) handleContactsByLocation(ctx context.Context, request *mcp.CallToolRequest, params locationParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "contactsByLocation")

	logger.Debug("Search google contacts by location", "query", params.Query)
	result, err := m.searchContactsByLocation(ctx, config, params)
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

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.TokenFile == "" {
		return errors.New("googlecontacts token file is not configured")
	}
	return nil
}

// searchContacts now accepts a slice of strings
func (m *Module) searchContacts(ctx context.Context, config Config, query []string) (string, error) {
	client := getClient(&config)

	srv, err := people.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("unable to retrieve people client: %w", err)
	}

	// If query is empty, list connections instead of searching
	if len(query) == 0 {
		return listConnections(srv)
	}

	// We'll collect results from all queries
	var allResults []*people.Person

	for _, q := range query {
		if q == "" {
			continue
		}
		call := srv.People.SearchContacts().Query(q).ReadMask(strings.Join(readMask, ","))

		r, err := call.Do()
		if err != nil {
			// Log error but continue with other queries?
			// For now, let's fail if one fails, or we could log and continue
			return "", fmt.Errorf("unable to search contacts for query %q: %w", q, err)
		}
		for _, res := range r.Results {
			allResults = append(allResults, res.Person)
		}
	}

	if len(allResults) == 0 {
		return "No contacts found.", nil
	}

	// Remove duplicates based on ResourceName
	uniqueResults := make([]*people.Person, 0, len(allResults))
	seen := make(map[string]bool)
	for _, p := range allResults {
		if !seen[p.ResourceName] {
			seen[p.ResourceName] = true
			uniqueResults = append(uniqueResults, p)
		}
	}

	j, err := json.Marshal(uniqueResults)
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

func (m *Module) searchContactsByDate(ctx context.Context, config Config, start, end string) (string, error) {
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
		if matchesDate(person, start, end) {
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

func matchesDate(person *people.Person, start, end string) bool {
	for _, birthday := range person.Birthdays {
		if checkDate(birthday.Date, start, end) {
			return true
		}
	}
	for _, event := range person.Events {
		if checkDate(event.Date, start, end) {
			return true
		}
	}
	return false
}

func toMMDD(s string) string {
	if len(s) == 10 && s[4] == '-' {
		return s[5:]
	}
	return s
}

func checkDate(date *people.Date, startStr, endStr string) bool {
	if date == nil {
		return false
	}
	// Format as MM-DD
	d := fmt.Sprintf("%02d-%02d", date.Month, date.Day)

	start := toMMDD(startStr)
	end := toMMDD(endStr)

	if start == "" || end == "" {
		return false
	}

	if start <= end {
		// Normal range (e.g. 01-01 to 01-31)
		if d >= start && d <= end {
			return true
		}
	} else {
		// Wrap around range (e.g. 12-01 to 01-31)
		if d >= start || d <= end {
			return true
		}
	}

	return false
}

func (m *Module) searchContactsByLocation(ctx context.Context, config Config, params locationParams) (string, error) {
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

func matchesLocation(person *people.Person, queries []string) bool {
	for _, query := range queries {
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
	}
	return false
}
