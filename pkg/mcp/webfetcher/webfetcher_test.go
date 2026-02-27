package webfetcher

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/stretchr/testify/assert"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		keepTags  bool
		keepForms bool
		expected  string
	}{
		{
			name:     "Convert to markdown (Strip tags)",
			html:     "<html><body><h1>Title</h1><p>Some text</p></body></html>",
			keepTags: false,
			expected: "# Title\n\nSome text",
		},
		{
			name:     "Keep tags (Raw HTML)",
			html:     "<html><body><h1>Title</h1><p>Some text</p></body></html>",
			keepTags: true,
			expected: "<html><body><h1>Title</h1><p>Some text</p></body></html>",
		},
		{
			name:      "Keep forms",
			html:      `<html><body><h1>Login</h1><form action="/login"><input name="user"></form></body></html>`,
			keepTags:  false,
			keepForms: true,
			expected:  "# Login\n\n<form action=\"/login\"><input name=\"user\"/></form>",
		},
		{
			name:      "Strip forms if keepForms is false",
			html:      `<html><body><h1>Login</h1><form action="/login"><input name="user"></form></body></html>`,
			keepTags:  false,
			keepForms: false,
			expected:  "# Login", // Form content is stripped/converted. Inputs are usually void elements, might disappear or show up weirdly.
			// html-to-markdown default for form is to process children. Input usually becomes empty string.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock generic.GetBody
			originalGetBody := generic.GetBody
			defer func() { generic.GetBody = originalGetBody }()

			generic.GetBody = func(url string) ([]byte, error) {
				return []byte(tt.html), nil
			}

			m := New(Config{}, slog.Default())
			result, err := m.fetchAndProcess("http://example.com", tt.keepTags, tt.keepForms)
			assert.NoError(t, err)

			// Normalize whitespace for easier comparison
			result = strings.TrimSpace(result)
			tt.expected = strings.TrimSpace(tt.expected)

			assert.Equal(t, tt.expected, result)
		})
	}
}
