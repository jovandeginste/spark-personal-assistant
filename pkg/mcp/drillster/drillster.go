package drillster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Drillster struct {
	mcp.BaseModule
	config Config
	client *http.Client
	token  string
}

func New(config Config, logger *slog.Logger) *Drillster {
	jar, _ := cookiejar.New(nil)
	return &Drillster{
		BaseModule: mcp.NewBaseModule(config, logger),
		config:     config,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
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

func (d *Drillster) login() error {
	// Step 1: GET /daas/identify
	resp, err := d.client.Get("https://www.drillster.com/daas/identify")
	if err != nil {
		return fmt.Errorf("failed identify step 1: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read identify body: %w", err)
	}

	// Extract CSRF token
	re := regexp.MustCompile(`name="_csrf" value="([^"]+)"`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return errors.New("csrf token not found")
	}
	csrf := string(matches[1])

	// Step 2: POST /daas/identify
	form := url.Values{}
	form.Add("username", d.config.Username)
	form.Add("_csrf", csrf)

	resp, err = d.client.PostForm("https://www.drillster.com/daas/identify", form)
	if err != nil {
		return fmt.Errorf("failed identify step 2: %w", err)
	}
	defer resp.Body.Close()

	// Step 3: POST /daas/authenticate/password
	form = url.Values{}
	form.Add("principalIdentifierAccountId", "")
	form.Add("password", d.config.Password)
	form.Add("_csrf", csrf)

	resp, err = d.client.PostForm("https://www.drillster.com/daas/authenticate/password", form)
	if err != nil {
		return fmt.Errorf("failed authenticate: %w", err)
	}
	defer resp.Body.Close()

	// Step 4: GET /tmb/token
	resp, err = d.client.Get("https://www.drillster.com/tmb/token")
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	tokenBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token body: %w", err)
	}

	// Parse JSON response to get access_token
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(tokenBytes, &tokenResp); err != nil {
		// Fallback: try to treat body as token string if not JSON (legacy behavior check, though prompt said JSON)
		// But prompt explicitly showed JSON response.
		return fmt.Errorf("failed to parse token json: %w", err)
	}

	d.token = tokenResp.AccessToken
	if d.token == "" {
		return errors.New("empty token received")
	}

	return nil
}

func (d *Drillster) makeRequest(method, requestURL string) (*sdk.CallToolResult, any, error) {
	if d.token == "" {
		if err := d.login(); err != nil {
			return &sdk.CallToolResult{
				Content: []sdk.Content{
					&sdk.TextContent{
						Text: fmt.Sprintf("Error logging in: %v", err),
					},
				},
				IsError: true,
			}, nil, nil
		}
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

	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
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

	// If unauthorized, maybe token expired? could retry login here.
	if resp.StatusCode == http.StatusUnauthorized {
		// Reset token so next request tries login again
		d.token = ""
		// Optionally we could retry once here, but let's keep it simple for now and let the next user turn retry.
		// Actually, let's retry once immediately for better UX.
		if err := d.login(); err == nil {
			req.Header.Set("Authorization", "Bearer "+d.token)
			if resp, err = d.client.Do(req); err == nil {
				defer resp.Body.Close()
			} else {
				// Retry failed
				return &sdk.CallToolResult{
					Content: []sdk.Content{
						&sdk.TextContent{
							Text: fmt.Sprintf("Error executing request after re-login: %v", err),
						},
					},
					IsError: true,
				}, nil, nil
			}
		}
	}

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
