package recycleapp

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/recycleapp-ics/pkg/recycleapp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRegister(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0"}, &mcp.ServerOptions{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	module := New(Config{}, logger)
	err := module.Register(server)
	assert.NoError(t, err)
}

func TestHandleGetCollections(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	module := New(Config{
		Zipcode:     1000,
		Street:      "Nieuwstraat",
		HouseNumber: 1,
		Lang:        "nl",
	}, logger)

	// Mock HTTP client transport to intercept the recycleapp CMS requests
	mockHTTPClient := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			query := req.URL.Query()

			var responseBody string
			switch {
			case path == "/recyclecms/public/v1/zipcodes":
				assert.Equal(t, "1000", query.Get("q"))
				responseBody = `{"items":[{"id":"zip-1234"}],"total":1,"pages":1,"page":1,"size":1}`
			case path == "/recyclecms/public/v1/streets":
				assert.Equal(t, "Nieuwstraat", query.Get("q"))
				assert.Equal(t, "zip-1234", query.Get("zipcodes"))
				responseBody = `{"items":[{"id":"street-5678"}],"total":1,"pages":1,"page":1,"size":1}`
			case path == "/recyclecms/public/v1/collections":
				assert.Equal(t, "zip-1234", query.Get("zipcodeId"))
				assert.Equal(t, "street-5678", query.Get("streetId"))
				assert.Equal(t, "1", query.Get("houseNumber"))
				assert.Equal(t, "2026-06-18", query.Get("fromDate"))
				assert.Equal(t, "2026-06-18", query.Get("untilDate"))
				responseBody = `{
					"items": [
						{
							"id": "collection-1",
							"type": "collection",
							"timestamp": "2026-06-18T00:00:00Z",
							"fraction": {
								"name": {"nl": "Huisvuil", "en": "Residual Waste"},
								"color": "blue",
								"createdAt": "2026-06-18T00:00:00Z",
								"updatedAt": "2026-06-18T00:00:00Z"
							},
							"exception": {
								"replacedBy": {"type": ""},
								"replaces": {"type": ""}
							}
						}
					],
					"page": 1,
					"total": 1,
					"size": 1
				}`
			default:
				t.Fatalf("Unexpected request path: %s", path)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	module.client = recycleapp.NewClient(mockHTTPClient)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	// Query with specific date
	params := collectionsParams{
		Zipcode:     1000,
		Street:      "Nieuwstraat",
		HouseNumber: 1,
		Lang:        "nl",
		DateRangeParams: sparkmcp.DateRangeParams{
			Date: "2026-06-18",
		},
	}

	res, meta, err := module.handleGetCollections(ctx, req, params)
	assert.NoError(t, err)
	assert.Nil(t, meta)
	assert.NotNil(t, res)
	assert.Len(t, res.Content, 1)

	textContent, ok := res.Content[0].(*mcp.TextContent)
	assert.True(t, ok)
	assert.Contains(t, textContent.Text, `"summary":"Huisvuil"`)
	assert.Contains(t, textContent.Text, `"date":"2026-06-18"`)
	assert.Contains(t, textContent.Text, `"color":"blue"`)
}
