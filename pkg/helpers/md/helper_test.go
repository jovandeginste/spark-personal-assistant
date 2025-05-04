package md

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMDFileToHTML(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		filename     string
		fileContent  string // Content to write to the file
		createFile   bool   // Whether to actually create the file
		expectedHTML string
		expectError  bool
	}{
		{
			name:         "Valid Markdown File",
			filename:     "test_valid.md",
			fileContent:  "# Hello\n\n* List item",
			createFile:   true,
			expectedHTML: "<h1 id=\"hello\">Hello</h1>\n\n<ul>\n<li>List item</li>\n</ul>\n",
			expectError:  false,
		},
		{
			name:         "Empty File",
			filename:     "test_empty.md",
			fileContent:  "",
			createFile:   true,
			expectedHTML: "", // markdown.GenerateHTML returns empty for empty input
			expectError:  false,
		},
		{
			name:         "File with Whitespace",
			filename:     "test_whitespace.md",
			fileContent:  "  \n\n",
			createFile:   true,
			expectedHTML: "", // markdown.GenerateHTML returns empty for just whitespace
			expectError:  false,
		},
		{
			name:         "Non-existent File",
			filename:     "non_existent.md",
			fileContent:  "", // Not used as file is not created
			createFile:   false,
			expectedHTML: "",   // Not used as error is expected
			expectError:  true, // Should return an os.PathError or similar
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)

			if tt.createFile {
				err := os.WriteFile(filePath, []byte(tt.fileContent), 0o600)
				require.NoError(t, err, "Failed to write test file")
			}

			html, err := MDFileToHTML(filePath)

			if tt.expectError {
				assert.Error(t, err, "Expected an error for filename: %s", tt.filename)
				// Check if the error is related to opening/reading the file
				// assert.True(t, os.IsNotExist(err), "Expected 'file not found' error for non-existent file") // This might be too specific, a generic error check is often enough
			} else {
				assert.NoError(t, err, "Did not expect an error for filename: %s", tt.filename)
				assert.Equal(t, tt.expectedHTML, html, "Generated HTML mismatch for file: %s", tt.filename)
			}

			// Clean up the created file if it exists
			if tt.createFile {
				os.Remove(filePath) // Errors on Remove during tests are usually not critical unless the test failed
			}
		})
	}
}
