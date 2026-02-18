package twizzit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	PHPSessionID string `mapstructure:"php_session_id"`
}

type Twizzit struct {
	mcp.BaseModule
	config Config
	client *http.Client
}

func New(config Config, logger *slog.Logger) *Twizzit {
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://app.twizzit.com")
	jar.SetCookies(u, []*http.Cookie{
		{
			Name:  "PHPSESSID",
			Value: config.PHPSessionID,
		},
	})

	return &Twizzit{
		BaseModule: mcp.NewBaseModule(config, logger),
		config:     config,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (t *Twizzit) Initialize() error {
	return nil
}

func (t *Twizzit) Register(server *sdk.Server) error {
	t.registerGetNotifications(server)
	t.registerGetSubscriptionForms(server)
	t.registerGetSubscriptionByFormId(server)
	return nil
}

type GetNotificationsParams struct {
	Amount int `json:"amount" jsonschema:"The number of notifications to retrieve"`
	Offset int `json:"offset" jsonschema:"The offset for pagination"`
}

func (t *Twizzit) registerGetNotifications(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_notifications",
		Description: "Get my notifications from Twizzit",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetNotificationsParams) (*sdk.CallToolResult, any, error) {
		amount := params.Amount
		if amount == 0 {
			amount = 20
		}
		u := fmt.Sprintf("https://app.twizzit.com/v2/ajax/my/notifications?amount=%d&offset=%d", amount, params.Offset)
		return t.makeRequest("GET", u)
	}

	sdk.AddTool(server, tool, handler)
}

type GetSubscriptionFormsParams struct {
	Archive bool `json:"archive" jsonschema:"Whether to retrieve archived forms (default: false)"`
}

func (t *Twizzit) registerGetSubscriptionForms(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_subscription_forms",
		Description: "Get the list of subscription forms",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetSubscriptionFormsParams) (*sdk.CallToolResult, any, error) {
		u := "https://app.twizzit.com/v2/ajax/forms"
		form := url.Values{}
		if params.Archive {
			form.Add("archive", "true")
		} else {
			form.Add("archive", "false")
		}
		form.Add("searchString", "")

		// Call makeRequest
		result, _, err := t.makeRequest("POST", u, form)
		if err != nil {
			return nil, nil, err
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit")
		}

		// The response is expected to be TextContent containing JSON
		textContent, ok := result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit")
		}

		// Unmarshal the outer JSON to extract the "forms" HTML string
		var response struct {
			Forms string `json:"forms"`
		}
		if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
		}

		// Parse the HTML content using our parser
		forms, err := parseSubscriptionForms(response.Forms)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse HTML forms: %w", err)
		}

		// Return the structured forms as JSON
		formsJSON, err := json.Marshal(forms)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal forms to JSON: %w", err)
		}

		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(formsJSON),
				},
			},
		}, nil, nil
	}

	sdk.AddTool(server, tool, handler)
}

type GetSubscriptionByFormIdParams struct {
	FormID int `json:"form_id" jsonschema:"The ID of the subscription form"`
}

type FormEntry struct {
	EntryID int    `json:"entry_id"`
	Content string `json:"content"`
}

func (t *Twizzit) registerGetSubscriptionByFormId(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_subscription_by_form_id",
		Description: "Get the list of subscriptions for a specific form",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetSubscriptionByFormIdParams) (*sdk.CallToolResult, any, error) {
		// 1. POST to /v2/ajax/form/{formID}/entry/query
		queryURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry/query", params.FormID)
		queryData := url.Values{}
		queryData.Set("filter[created-after]", "")
		queryData.Set("filter[created-before]", "")
		queryData.Set("incomplete", "0")

		result, _, err := t.makeRequest("POST", queryURL, queryData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to query entries: %w", err)
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit query")
		}

		textContent, ok := result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit query")
		}

		var queryResponse struct {
			SearchResultID string `json:"searchResultId"`
		}
		if err := json.Unmarshal([]byte(textContent.Text), &queryResponse); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal query response: %w", err)
		}

		if queryResponse.SearchResultID == "" {
			return nil, nil, errors.New("no searchResultId found in response")
		}

		// 2. GET to /v2/ajax/form/{formID}/entry/box?searchResultId=...
		boxURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry/box?searchResultId=%s", params.FormID, queryResponse.SearchResultID)
		result, _, err = t.makeRequest("GET", boxURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get entry box: %w", err)
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit entry box")
		}
		textContent, ok = result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit entry box")
		}

		entryIDs, err := parseEntryIDs(textContent.Text)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse entry IDs: %w", err)
		}

		// 4. Loop through IDs and fetch details
		var finalEntries []FormEntry
		for _, id := range entryIDs {
			// URL: /v2/ajax/form/{formID}/entry?entryId={entryID}
			entryURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry?entryId=%d", params.FormID, id)
			
			// We need to fetch the content. The makeRequest method handles the GET.
			result, _, err := t.makeRequest("GET", entryURL)
			if err != nil {
				// We can log this error if we had the logger easily accessible, or just skip.
				continue
			}
			
			if len(result.Content) > 0 {
				if tc, ok := result.Content[0].(*sdk.TextContent); ok {
					// 5. Sanitize HTML
					stripped := stripHTML(tc.Text)
					finalEntries = append(finalEntries, FormEntry{
						EntryID: id,
						Content: stripped,
					})
				}
			}
		}

		// 6. Return JSON list
		entriesJSON, err := json.Marshal(finalEntries)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal entries: %w", err)
		}

		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(entriesJSON),
				},
			},
		}, nil, nil
	}

	sdk.AddTool(server, tool, handler)
}

