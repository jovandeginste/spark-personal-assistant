package twizzit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchContactsParams struct {
	Query  string `json:"query" jsonschema:"The search term"`
	Offset int    `json:"offset,omitempty" jsonschema:"The offset for pagination (default: 1)"`
}

func (t *Twizzit) registerSearchContacts(server *sdk.Server) {
	tool := &sdk.Tool{
		Name:        "twizzit_search_contacts",
		Description: "Search for contacts in Twizzit",
	}

	handler := func(ctx context.Context, request *sdk.CallToolRequest, params SearchContactsParams) (*sdk.CallToolResult, any, error) {
		// Clean up the query: trim spaces and squash consecutive spaces
		params.Query = strings.Join(strings.Fields(params.Query), " ")

		// Default offset to 1
		offset := params.Offset
		if offset <= 0 {
			offset = 1
		}

		// 1. POST to /v2/ajax/clubmanager/search/query
		queryURL := "https://app.twizzit.com/v2/ajax/clubmanager/search/query"
		queryData := url.Values{}
		queryData.Set("view", "contact")
		queryData.Set("licenceRelationId", "")
		// searchFields needs to be a JSON string inside the form param
		searchFields := fmt.Sprintf(`{"textSearch":"%s"}`, params.Query)
		queryData.Set("searchFields", searchFields)

		t.Logger().Info("querying contacts", "url", queryURL, "data", queryData.Encode())

		result, _, err := t.makeRequest("POST", queryURL, queryData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to query contacts: %w", err)
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit contact query")
		}

		textContent, ok := result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit contact query")
		}

		var queryResponse struct {
			SearchResultID string `json:"searchResultId"`
		}
		if err := json.Unmarshal([]byte(textContent.Text), &queryResponse); err != nil {
			preview := textContent.Text
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			return nil, nil, fmt.Errorf("failed to unmarshal contact query response: %w (content preview: %s)", err, preview)
		}

		if queryResponse.SearchResultID == "" {
			return nil, nil, errors.New("no searchResultId found in contact query response")
		}

		// 2. POST to /v2/ajax/clubmanager/search/box
		boxURL := "https://app.twizzit.com/v2/ajax/clubmanager/search/box"
		boxData := url.Values{}
		boxData.Set("view", "contact")
		boxData.Set("searchResultId", queryResponse.SearchResultID)
		boxData.Set("boxId", strconv.Itoa(offset))
		boxData.Set("subscriberListId", "")
		// Add columns individually
		boxData.Add("columns[]", "id")
		boxData.Add("columns[]", "profile-image")
		boxData.Add("columns[]", "name")
		boxData.Add("columns[]", "first-name")
		boxData.Add("columns[]", "gender")
		boxData.Add("columns[]", "dob")
		boxData.Add("columns[]", "address")
		boxData.Add("columns[]", "shirtnumber")
		boxData.Add("columns[]", "email1")
		boxData.Add("columns[]", "mobile1")

		result, _, err = t.makeRequest("POST", boxURL, boxData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get contact results box: %w", err)
		}

		if len(result.Content) == 0 {
			return nil, nil, errors.New("empty response from Twizzit contact results box")
		}
		textContent, ok = result.Content[0].(*sdk.TextContent)
		if !ok {
			return nil, nil, errors.New("unexpected content type from Twizzit contact results box")
		}

		// 3. Parse HTML results
		contacts, err := parseContactSearchResults(textContent.Text)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse contact results: %w", err)
		}

		// 4. Return JSON list
		contactsJSON, err := json.Marshal(contacts)
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
