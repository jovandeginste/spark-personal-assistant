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
	"strings"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	OrganizationID int    `mapstructure:"organization_id"`
}

type Twizzit struct {
	mcp.BaseModule
	config Config
	client *http.Client
}

func New(config Config, logger *slog.Logger) *Twizzit {
	jar, _ := cookiejar.New(nil)

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
	t.registerGetActivitiesInfo(server)
	t.registerGetContactsInfo(server)
	t.registerGetEvents(server)
	t.registerSearchContacts(server)

	return nil
}

type GetActivitiesInfoParams struct {
	ActivityIDs []int `json:"activity_ids" jsonschema:"The IDs of the activities"`
}

func (t *Twizzit) registerGetActivitiesInfo(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_activities_info",
		Description: "Get details for specific activities, including attendance",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetActivitiesInfoParams) (*sdk.CallToolResult, any, error) {
		var allDetails []any

		for _, id := range params.ActivityIDs {
			u := fmt.Sprintf("https://app.twizzit.com/v2/activity/details?activity=%d&view=info", id)
			result, _, err := t.makeRequest("GET", u)
			if err != nil {
				// We skip errors for individual activities to allow partial success
				continue
			}

			if len(result.Content) == 0 {
				continue
			}

			textContent, ok := result.Content[0].(*sdk.TextContent)
			if !ok {
				continue
			}

			details, err := parseActivityDetails(textContent.Text)
			if err != nil {
				continue
			}
			allDetails = append(allDetails, details)
		}

		detailsJSON, err := json.Marshal(allDetails)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal activity details: %w", err)
		}

		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(detailsJSON),
				},
			},
		}, nil, nil
	}

	sdk.AddTool(server, tool, handler)
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
		entries, err := t.fetchSubscriptionEntries(params.FormID)
		if err != nil {
			return nil, nil, err
		}

		entriesJSON, err := json.Marshal(entries)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal entries: %w", err)
		}

		return textResult(string(entriesJSON), false), nil, nil
	}

	sdk.AddTool(server, tool, handler)
}

func (t *Twizzit) fetchSubscriptionEntries(formID int) ([]FormEntry, error) {
	searchResultID, err := t.fetchSearchResultID(formID)
	if err != nil {
		return nil, err
	}

	entryIDs, err := t.fetchEntryIDs(formID, searchResultID)
	if err != nil {
		return nil, err
	}

	return t.fetchEntryDetails(formID, entryIDs), nil
}

func (t *Twizzit) fetchSearchResultID(formID int) (string, error) {
	queryURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry/query", formID)
	queryData := url.Values{}
	queryData.Set("filter[created-after]", "")
	queryData.Set("filter[created-before]", "")
	queryData.Set("incomplete", "0")

	result, _, err := t.makeRequest("POST", queryURL, queryData)
	if err != nil {
		return "", fmt.Errorf("failed to query entries: %w", err)
	}

	body, err := getResultText(result)
	if err != nil {
		return "", errors.New("empty response from Twizzit query")
	}

	var queryResponse struct {
		SearchResultID string `json:"searchResultId"`
	}
	if err := json.Unmarshal([]byte(body), &queryResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal query response: %w", err)
	}

	if queryResponse.SearchResultID == "" {
		return "", errors.New("no searchResultId found in response")
	}

	return queryResponse.SearchResultID, nil
}

func (t *Twizzit) fetchEntryIDs(formID int, searchResultID string) ([]int, error) {
	boxURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry/box?searchResultId=%s", formID, searchResultID)
	result, _, err := t.makeRequest("GET", boxURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry box: %w", err)
	}

	body, err := getResultText(result)
	if err != nil {
		return nil, errors.New("empty response from Twizzit entry box")
	}

	entryIDs, err := parseEntryIDs(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse entry IDs: %w", err)
	}

	return entryIDs, nil
}

func (t *Twizzit) fetchEntryDetails(formID int, entryIDs []int) []FormEntry {
	entries := make([]FormEntry, 0, len(entryIDs))

	for _, id := range entryIDs {
		entryURL := fmt.Sprintf("https://app.twizzit.com/v2/ajax/form/%d/entry?entryId=%d", formID, id)
		entryResult, _, err := t.makeRequest("GET", entryURL)
		if err != nil {
			continue
		}

		entryText, err := getResultText(entryResult)
		if err != nil {
			continue
		}

		entries = append(entries, FormEntry{
			EntryID: id,
			Content: stripHTML(entryText),
		})
	}

	return entries
}

func getResultText(result *sdk.CallToolResult) (string, error) {
	if len(result.Content) == 0 {
		return "", errors.New("empty response")
	}

	textContent, ok := result.Content[0].(*sdk.TextContent)
	if !ok {
		return "", errors.New("unexpected content type")
	}

	return textContent.Text, nil
}

type GetEventsParams struct {
	Limit  int `json:"limit" jsonschema:"The number of events to retrieve (0..50). Default is 50."`
	Offset int `json:"offset" jsonschema:"The offset to start retrieving events from. Default is 0."`
}

func (t *Twizzit) registerGetEvents(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_events",
		Description: "Get planned or past events (activities) from Twizzit feed",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetEventsParams) (*sdk.CallToolResult, any, error) {
		limit := params.Limit
		switch {
		case limit <= 0:
			limit = 50
		case limit > 50:
			limit = 50
		}

		offset := params.Offset
		if offset < 0 {
			offset = 0
		}

		u := fmt.Sprintf("https://app.twizzit.com/v2/ajax/feed?limit=%d&offset=%d", limit, offset)
		result, _, err := t.makeRequest("GET", u)
		if err != nil {
			return nil, nil, err
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit")
		}

		textContent, ok := result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit")
		}

		events, err := parseEventsFeed(textContent.Text)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse events feed: %w", err)
		}

		eventsJSON, err := json.Marshal(events)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal events: %w", err)
		}

		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(eventsJSON),
				},
			},
		}, nil, nil
	}

	sdk.AddTool(server, tool, handler)
}

type GetContactsInfoParams struct {
	ContactIDs []int `json:"contact_ids" jsonschema:"The IDs of the contacts to retrieve"`
}

type ContactInfo struct {
	ID   int    `json:"id"`
	Info string `json:"info"`
}

func (t *Twizzit) registerGetContactsInfo(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_get_contacts_info",
		Description: "Get details for specific contacts",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params GetContactsInfoParams) (*sdk.CallToolResult, any, error) {
		var allContacts []ContactInfo

		for _, id := range params.ContactIDs {
			u := fmt.Sprintf("https://app.twizzit.com/v2/ajax/clubmanager/profile/contact/modal?contactId=%d", id)
			result, _, err := t.makeRequest("GET", u)
			if err != nil {
				continue
			}

			if len(result.Content) == 0 {
				continue
			}

			textContent, ok := result.Content[0].(*sdk.TextContent)
			if !ok {
				continue
			}

			// Clean the content: strip HTML and collapse whitespaces
			cleaned := stripHTML(textContent.Text)
			// Collapse multiple whitespaces/newlines into single space or sensible format?
			// The requirement says "collapses newlines and whitespaces".
			// stripHTML already does some cleaning, let's refine it.
			// Let's replace all whitespace sequences with a single space to be safe,
			// or keep newlines if they are meaningful.
			// The prompt says "collapses newlines and whitespaces", usually meaning `\s+` -> ` `.
			fields := strings.Fields(cleaned)
			finalContent := strings.Join(fields, " ")

			allContacts = append(allContacts, ContactInfo{
				ID:   id,
				Info: finalContent,
			})
		}

		contactsJSON, err := json.Marshal(allContacts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal contacts: %w", err)
		}

		return &sdk.CallToolResult{
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: string(contactsJSON),
				},
			},
		}, nil, nil
	}

	sdk.AddTool(server, tool, handler)
}
