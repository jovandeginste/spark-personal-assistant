package webfetcher

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

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

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		// If keepForms is true, and we encounter a form element, we want to keep it and its children as HTML
		if keepForms && isFormElement(n) {
			// Render this node and its children as HTML
			var buf bytes.Buffer
			if err := html.Render(&buf, n); err == nil {
				sb.WriteString("\n")
				sb.WriteString(buf.String())
				sb.WriteString("\n")
			}
			return // Don't traverse children, we already rendered them
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}

		// Don't traverse into script or style tags if we are stripping tags
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		// Add newlines for block elements to maintain readability
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
				sb.WriteString("\n")
			}
		}
	}
	f(doc)

	return strings.TrimSpace(sb.String()), nil
}

func isFormElement(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	return n.Data == "form"
}
