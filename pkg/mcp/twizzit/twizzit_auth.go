package twizzit

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *Twizzit) makeRequest(method, requestURL string, formData ...url.Values) (*sdk.CallToolResult, any, error) {
	t.Logger().Debug("making request to twizzit", "method", method, "url", requestURL)

	var body io.Reader
	if len(formData) > 0 {
		body = strings.NewReader(formData[0].Encode())
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		location, err := resp.Location()
		if err == nil {
			if location.String() == "https://app.twizzit.com/v2/login?expired=1" {
				return nil, nil, errors.New("twizzit session expired")
			}
		}
	}

	respBody, err := io.ReadAll(resp.Body)

	return &sdk.CallToolResult{
		Content: []sdk.Content{
			&sdk.TextContent{
				Text: string(respBody),
			},
		},
	}, nil, err
}
