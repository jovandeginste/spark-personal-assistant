package drillster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	TokenFile string `mapstructure:"token_file"`
}

type Drillster struct {
	mcp.BaseModule
	config Config
}

func New(config Config, logger *slog.Logger) *Drillster {
	return &Drillster{
		BaseModule: mcp.NewBaseModule(config, logger),
		config:     config,
	}
}

func (d *Drillster) Initialize() error {
	return nil
}

func (d *Drillster) Register(server *sdk.Server) error {
	d.registerWhoAmI(server)
	d.registerSearchMembers(server)
	d.registerGetTestResults(server)
	return nil
}

type GetTestResultsParams struct {
	MemberID string `json:"member_id" jsonschema:"The ID of the member to get results for"`
}

func (d *Drillster) registerGetTestResults(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "drillster_get_test_results",
		Description: "Get test results for a specific member",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetTestResultsParams) (*sdk.CallToolResult, any, error) {
		u := fmt.Sprintf("https://www.drillster.com/api/3/group-member-results/%s/tests", params.MemberID)
		return d.makeRequest("GET", u)
	}

	sdk.AddTool(server, tool, handler)
}

func (d *Drillster) registerWhoAmI(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "drillster_whoami",
		Description: "Get current user info from Drillster",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, arguments map[string]any) (*sdk.CallToolResult, any, error) {
		return d.makeRequest("GET", "https://www.drillster.com/api/3/persons/self/accounts")
	}

	sdk.AddTool(server, tool, handler)
}

type SearchMembersParams struct {
	Name string `json:"name" jsonschema:"The name to search for"`
	Size int    `json:"size" jsonschema:"The number of results to return"`
}

func (d *Drillster) registerSearchMembers(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "drillster_search_members",
		Description: "Search for members in Drillster",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params SearchMembersParams) (*sdk.CallToolResult, any, error) {
		u, _ := url.Parse("https://www.drillster.com/api/3/groups/members")
		q := u.Query()
		q.Set("query", params.Name)
		if params.Size > 0 {
			q.Set("resultSize", strconv.Itoa(params.Size))
		}
		u.RawQuery = q.Encode()

		return d.makeRequest("GET", u.String())
	}

	sdk.AddTool(server, tool, handler)
}

func (d *Drillster) makeRequest(method, requestURL string) (*sdk.CallToolResult, any, error) {
	token, err := d.readToken()
	if err != nil {
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: fmt.Sprintf("Error reading token: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(method, requestURL, nil)
	if err != nil {
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: fmt.Sprintf("Error creating request: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: fmt.Sprintf("Error executing request: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: fmt.Sprintf("Error reading response body: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}

	return &sdk.CallToolResult{
		Content: []sdk.Content{
			&sdk.TextContent{
				Text: string(body),
			},
		},
	}, nil, nil
}

func (d *Drillster) readToken() (string, error) {
	if d.config.TokenFile == "" {
		return "", errors.New("token_file not configured")
	}

	data, err := os.ReadFile(d.config.TokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read token file %s: %w", d.config.TokenFile, err)
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.Unmarshal(data, &tokenData); err != nil {
		return "", fmt.Errorf("failed to parse token file: %w", err)
	}

	if tokenData.AccessToken == "" {
		return "", errors.New("access_token not found in token file")
	}

	return tokenData.AccessToken, nil
}
