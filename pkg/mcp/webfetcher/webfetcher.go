package webfetcher

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

type Config struct {
	// Add config fields if needed, e.g. UserAgent or timeout
}

type webFetcherParams struct {
	URL       string `json:"url" jsonschema:"The URL to fetch"`
	KeepTags  bool   `json:"keep_tags,omitempty" jsonschema:"Whether to return the raw HTML tags (default: false)"`
	KeepForms bool   `json:"keep_forms,omitempty" jsonschema:"Whether to keep HTML forms when stripping tags (default: false)"`
}

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	m := &Module{}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "webfetcher"))
	return m
}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()

	tool := &mcp.Tool{
		Name:        "webfetcher",
		Description: "Fetch a web page by URL, optionally stripping HTML tags",
	}

	handler := func(ctx context.Context, request *mcp.CallToolRequest, params webFetcherParams) (*mcp.CallToolResult, any, error) {
		reqLogger := logger.With("handler", "webfetcher")
		reqLogger.Debug("Fetching web page", "url", params.URL, "keep_tags", params.KeepTags, "keep_forms", params.KeepForms)

		content, err := m.fetchAndProcess(params.URL, params.KeepTags, params.KeepForms)
		if err != nil {
			reqLogger.Error("Failed to fetch web page", "url", params.URL, "error", err)
			return nil, nil, err
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: content,
				},
			},
		}, nil, nil
	}

	mcp.AddTool(server, tool, handler)

	return nil
}

func (m *Module) Enabled() error {
	return nil
}

func (m *Module) fetchAndProcess(url string, keepTags, keepForms bool) (string, error) {
	// Use the generic helper to get the body
	body, err := generic.GetBody(url)
	if err != nil {
		return "", err
	}

	if keepTags {
		return string(body), nil
	}

	converter := md.NewConverter("", true, nil)

	if keepForms {
		converter.AddRules(md.Rule{
			Filter: []string{"form"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				// Render the outer HTML of the form
				var buf bytes.Buffer
				if len(selec.Nodes) > 0 {
					if err := html.Render(&buf, selec.Nodes[0]); err == nil {
						s := buf.String()
						return &s
					}
				}
				return nil
			},
		})
	}

	markdown, err := converter.ConvertBytes(body)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
	}

	return strings.TrimSpace(string(markdown)), nil
}
