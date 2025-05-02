package data

import (
	"bytes"
	"strings"
	"testing"
)

func TestSources_PrintTo(t *testing.T) {
	tests := []struct {
		name    string
		sources Sources
		want    string
	}{
		{
			name:    "Empty Sources",
			sources: Sources{},
			want: `┌────┬──────┬─────────────┐
│ ID │ Name │ Description │
└────┴──────┴─────────────┘`,
		},
		{
			name: "Single Source",
			sources: Sources{
				{ID: 1, Name: "Test1", Description: "Description 1"},
			},
			want: `┌────┬───────┬───────────────┐
│ ID │ Name  │  Description  │
├────┼───────┼───────────────┤
│ 1  │ Test1 │ Description 1 │
└────┴───────┴───────────────┘`,
		},
		{
			name: "Multiple Sources",
			sources: Sources{
				{ID: 1, Name: "Test1", Description: "Description 1"},
				{ID: 2, Name: "Test2", Description: "Description 2"},
				{ID: 3, Name: "LongName", Description: "A rather long description"},
			},
			want: `┌────┬──────────┬───────────────────────────┐
│ ID │   Name   │        Description        │
├────┼──────────┼───────────────────────────┤
│ 1  │ Test1    │ Description 1             │
├────┼──────────┼───────────────────────────┤
│ 2  │ Test2    │ Description 2             │
├────┼──────────┼───────────────────────────┤
│ 3  │ LongName │ A rather long description │
└────┴──────────┴───────────────────────────┘`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tt.sources.PrintTo(&buf)

			// Trim trailing newline from table output for consistent comparison
			got := strings.TrimSuffix(buf.String(), "\n")
			want := strings.TrimSuffix(tt.want, "\n")

			if got != want {
				t.Errorf("Sources.PrintTo() got =\n%v\nwant =\n%v", got, want)
			}
		})
	}
}

func TestSource_PrintTo(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   string
	}{
		{
			name: "Standard Source",
			source: Source{
				ID:          1,
				Name:        "MySource",
				Description: "This is my test source",
			},
			want: `┌─────────────┬────────────────────────┐
│ Name        │ MySource               │
├─────────────┼────────────────────────┤
│ Description │ This is my test source │
└─────────────┴────────────────────────┘`,
		},
		{
			name: "Empty Description",
			source: Source{
				ID:   2,
				Name: "AnotherSource",
			},
			want: `┌─────────────┬───────────────┐
│ Name        │ AnotherSource │
├─────────────┼───────────────┤
│ Description │               │
└─────────────┴───────────────┘`,
		},
		{
			name: "Longer Fields",
			source: Source{
				ID:          3,
				Name:        "SourceWithLongName",
				Description: "A very long description that should make the table wider",
			},
			want: `┌─────────────┬──────────────────────────────────────────────────────────┐
│ Name        │ SourceWithLongName                                       │
├─────────────┼──────────────────────────────────────────────────────────┤
│ Description │ A very long description that should make the table wider │
└─────────────┴──────────────────────────────────────────────────────────┘`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tt.source.PrintTo(&buf)

			// Trim trailing newline from table output for consistent comparison
			got := strings.TrimSuffix(buf.String(), "\n")
			want := strings.TrimSuffix(tt.want, "\n")

			if got != want {
				t.Errorf("Source.PrintTo() got =\n%v\nwant =\n%v", got, want)
			}
		})
	}
}
