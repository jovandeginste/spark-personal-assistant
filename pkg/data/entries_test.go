//nolint:funlen
package data

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aquasecurity/table" // Required by the original code
)

// Define dummy types for testing purposes that match the usage in PrintTo

func TestEntries_PrintTo(t *testing.T) {
	tests := []struct {
		name        string
		entries     Entries
		expectedOut string
	}{
		{
			name:    "empty entries",
			entries: Entries{},
			expectedOut: `┌────┬──────┬───────┬────────────┬────────┐
│ ID │ Date │ Title │ Importance │ Source │
└────┴──────┴───────┴────────────┴────────┘`, // Empty table with headers
		},
		{
			name: "multiple entries",
			entries: Entries{
				{
					ID:         1,
					DateString: "2023-10-26",
					Summary:    "Test Summary 1",
					Importance: "High",
					Source: &Source{
						Name: "Source A",
					},
				},
				{
					ID:         2,
					DateString: "2023-10-27",
					Summary:    "Test Summary 2",
					Importance: "Medium",
					Source: &Source{
						Name: "Source B",
					},
				},
				{
					ID:         3,
					DateString: "2023-10-28",
					Summary:    "Another test entry",
					Importance: "Low",
					Source: &Source{
						Name: "Source A",
					},
				},
			},
			expectedOut: `┌────┬────────────┬────────────────────┬────────────┬──────────┐
│ ID │    Date    │       Title        │ Importance │  Source  │
├────┼────────────┼────────────────────┼────────────┼──────────┤
│ 1  │ 2023-10-26 │ Test Summary 1     │ High       │ Source A │
├────┼────────────┼────────────────────┼────────────┼──────────┤
│ 2  │ 2023-10-27 │ Test Summary 2     │ Medium     │ Source B │
├────┼────────────┼────────────────────┼────────────┼──────────┤
│ 3  │ 2023-10-28 │ Another test entry │ Low        │ Source A │
└────┴────────────┴────────────────────┴────────────┴──────────┘`,
		},
		{
			name: "single entry",
			entries: Entries{
				{
					ID:         10,
					DateString: "2024-01-01",
					Summary:    "Single Entry Test",
					Importance: "Critical",
					Source: &Source{
						Name: "Solo Source",
					},
				},
			},
			expectedOut: `┌────┬────────────┬───────────────────┬────────────┬─────────────┐
│ ID │    Date    │       Title       │ Importance │   Source    │
├────┼────────────┼───────────────────┼────────────┼─────────────┤
│ 10 │ 2024-01-01 │ Single Entry Test │ Critical   │ Solo Source │
└────┴────────────┴───────────────────┴────────────┴─────────────┘`,
		},
		{
			name: "entry with empty fields",
			entries: Entries{
				{
					ID:         99,
					DateString: "",
					Summary:    "",
					Importance: "",
					Source: &Source{
						Name: "",
					},
				},
			},
			expectedOut: `┌────┬──────┬───────┬────────────┬────────┐
│ ID │ Date │ Title │ Importance │ Source │
├────┼──────┼───────┼────────────┼────────┤
│ 99 │      │       │            │        │
└────┴──────┴───────┴────────────┴────────┘`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tt.entries.PrintTo(&buf)

			// The aquasecurity/table library adds a newline at the end.
			// We should ensure our expected string also ends with a newline.
			actualOut := buf.String()
			if !strings.HasSuffix(tt.expectedOut, "\n") && actualOut != "" {
				actualOut = strings.TrimRight(actualOut, "\n")
			}

			if actualOut != tt.expectedOut {
				t.Errorf("PrintTo() output mismatch\nExpected:\n%s\nActual:\n%s", tt.expectedOut, actualOut)
			}
		})
	}
}

// Helper function to programmatically generate the empty table string for comparison
// This is more robust than hardcoding the empty table string.
func generateEmptyTableString() string {
	var buf bytes.Buffer
	t := table.New(&buf)
	t.AddHeaders("ID", "Date", "Title", "Importance", "Source")
	t.Render()

	return buf.String()
}

func TestEntries_PrintTo_ProgrammaticEmpty(t *testing.T) {
	// This test explicitly uses programmatic generation for the empty case
	t.Run("empty entries programmatic", func(t *testing.T) {
		entries := Entries{}

		var buf bytes.Buffer

		entries.PrintTo(&buf)

		expectedOut := generateEmptyTableString()
		actualOut := buf.String()

		if actualOut != expectedOut {
			t.Errorf("PrintTo() output mismatch for empty entries\nExpected:\n%s\nActual:\n%s", expectedOut, actualOut)
		}
	})
}
