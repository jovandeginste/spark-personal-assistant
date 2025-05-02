package markdown

import (
	"bytes"
	"testing"
)

func TestGenerateHTML(t *testing.T) {
	tests := []struct {
		name          string
		markdownInput []byte
		expectedHTML  []byte
	}{
		{
			name:          "Basic Paragraph",
			markdownInput: []byte("This is a paragraph."),
			expectedHTML:  []byte("<p>This is a paragraph.</p>\n"),
		},
		{
			name:          "Heading Level 1",
			markdownInput: []byte("# Heading 1"),
			expectedHTML:  []byte("<h1 id=\"heading-1\">Heading 1</h1>\n"),
		},
		{
			name:          "Heading Level 2 with spaces",
			markdownInput: []byte("## Another Heading"),
			expectedHTML:  []byte("<h2 id=\"another-heading\">Another Heading</h2>\n"),
		},
		{
			name:          "Bold Text",
			markdownInput: []byte("This is **bold** text."),
			expectedHTML:  []byte("<p>This is <strong>bold</strong> text.</p>\n"),
		},
		{
			name:          "Italic Text",
			markdownInput: []byte("This is *italic* text."),
			expectedHTML:  []byte("<p>This is <em>italic</em> text.</p>\n"),
		},
		{
			name:          "Unordered List",
			markdownInput: []byte("* Item 1\n* Item 2"),
			expectedHTML:  []byte("<ul>\n<li>Item 1</li>\n<li>Item 2</li>\n</ul>\n"),
		},
		{
			name:          "Ordered List",
			markdownInput: []byte("1. First item\n2. Second item"),
			expectedHTML:  []byte("<ol>\n<li>First item</li>\n<li>Second item</li>\n</ol>\n"),
		},
		{
			name:          "Link with HrefTargetBlank",
			markdownInput: []byte("[Google](http://www.google.com)"),
			expectedHTML:  []byte("<p><a href=\"http://www.google.com\" target=\"_blank\">Google</a></p>\n"),
		},
		{
			name:          "Image",
			markdownInput: []byte("![Alt text](/path/to/img.jpg)"),
			expectedHTML:  []byte("<p><img src=\"/path/to/img.jpg\" alt=\"Alt text\" /></p>\n"),
		},
		{
			name:          "Code Block",
			markdownInput: []byte("```go\nfmt.Println(\"Hello, world!\")\n```"),
			expectedHTML:  []byte("<pre><code class=\"language-go\">fmt.Println(&quot;Hello, world!&quot;)\n</code></pre>\n"),
		},
		{
			name:          "Empty Input",
			markdownInput: []byte(""),
			expectedHTML:  []byte(""), // gomarkdown returns empty byte slice for empty input
		},
		{
			name:          "Input with spaces/newlines only",
			markdownInput: []byte("  \n\n"),
			expectedHTML:  []byte(""),
		},
		{
			name:          "Horizontal Rule",
			markdownInput: []byte("---\n\n***"),
			expectedHTML:  []byte("<hr>\n\n<hr>\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualHTML, err := GenerateHTML(tt.markdownInput)
			if err != nil {
				t.Fatalf("GenerateHTML returned an unexpected error: %v", err)
			}

			if !bytes.Equal(actualHTML, tt.expectedHTML) {
				t.Errorf("GenerateHTML(%q) returned %q, expected %q",
					tt.markdownInput, actualHTML, tt.expectedHTML)
			}
		})
	}
}
