package md

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock markdown.GenerateHTML behavior for isolation, if needed.
// However, since markdown.GenerateHTML is already tested and is the core
// conversion logic, testing the helper functions means testing if they
// correctly read input and pass it to GenerateHTML, and handle file operations.
// We can rely on the real markdown.GenerateHTML for these tests.

func TestMDToHTML(t *testing.T) {
	tests := []struct {
		name          string
		inputMarkdown string
		expectedHTML  string
		expectError   bool
	}{
		{
			name:          "Valid Markdown",
			inputMarkdown: "# Title\n\nThis is a paragraph.",
			// Expected output assuming markdown.GenerateHTML works as tested in markdown_test.go
			expectedHTML: "<h1 id=\"title\">Title</h1>\n\n<p>This is a paragraph.</p>\n",
			expectError:  false,
		},
		{
			name:          "Empty Input",
			inputMarkdown: "",
			expectedHTML:  "", // markdown.GenerateHTML returns empty for empty input
			expectError:   false,
		},
		{
			name:          "Whitespace and Newlines Only",
			inputMarkdown: "   \n\n\t",
			expectedHTML:  "", // markdown.GenerateHTML returns empty for just whitespace
			expectError:   false,
		},
		// Note: Simulating a read error from io.Reader directly is complex
		// without a custom test reader. Assuming valid reader input behavior.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.inputMarkdown)
			html, err := MDToHTML(reader)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.Equal(t, tt.expectedHTML, html, "Generated HTML mismatch")
			}
		})
	}
}
