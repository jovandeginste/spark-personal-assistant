package jsonreader

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

// Mock data
var mockEventsJSON = []byte(`[
	{
		"Date": "2026-01-10T12:00:00Z",
		"Summary": "Team Meeting",
		"Description": "Weekly sync with the team."
	},
	{
		"Date": "2026-01-15T09:00:00Z",
		"Summary": "Doctor Appointment",
		"Description": "Routine checkup."
	},
	{
		"Date": "2026-02-01T18:00:00Z",
		"Summary": "Dinner",
		"Description": "Dinner with friends."
	}
]`)

func TestFilterEventsByDate(t *testing.T) {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(Config{}, slog.Default()),
	}

	events := []Event{
		{Date: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC), Summary: "Event 1"},
		{Date: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC), Summary: "Event 2"},
		{Date: time.Date(2026, 2, 1, 18, 0, 0, 0, time.UTC), Summary: "Event 3"},
	}

	tests := []struct {
		name          string
		params        dateParams
		expectedCount int
	}{
		{
			name: "All events in range",
			params: dateParams{sparkmcp.DateRangeParams{
				StartDate: "2026-01-01",
				EndDate:   "2026-02-28",
			}},
			expectedCount: 3,
		},
		{
			name: "One event in range",
			params: dateParams{sparkmcp.DateRangeParams{
				StartDate: "2026-01-01",
				EndDate:   "2026-01-12",
			}},
			expectedCount: 1, // Only Event 1
		},
		{
			name: "No events in range",
			params: dateParams{sparkmcp.DateRangeParams{
				StartDate: "2025-01-01",
				EndDate:   "2025-12-31",
			}},
			expectedCount: 0,
		},
		{
			name: "Start date only",
			params: dateParams{sparkmcp.DateRangeParams{
				StartDate: "2026-01-14",
			}},
			expectedCount: 2, // Event 2 and 3
		},
		{
			name: "End date only",
			params: dateParams{sparkmcp.DateRangeParams{
				EndDate: "2026-01-12",
			}},
			expectedCount: 1, // Event 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := module.filterEventsByDate(events, tt.params)
			assert.Len(t, results, tt.expectedCount)
		})
	}
}

func TestFilterEventsByQuery(t *testing.T) {
	module := &Module{
		BaseModule: sparkmcp.NewBaseModule(Config{}, slog.Default()),
	}

	events := []Event{
		{Summary: "Team Meeting", Description: "Weekly sync"},
		{Summary: "Doctor Appointment", Description: "Checkup"},
		{Summary: "Project Deadline", Description: "Urgent"},
	}

	tests := []struct {
		name          string
		query         []string
		expectedCount int
	}{
		{
			name:          "Match summary",
			query:         []string{"Meeting"},
			expectedCount: 1,
		},
		{
			name:          "Match description",
			query:         []string{"Checkup"},
			expectedCount: 1,
		},
		{
			name:          "Case insensitive",
			query:         []string{"urgent"},
			expectedCount: 1,
		},
		{
			name:          "No match",
			query:         []string{"Vacation"},
			expectedCount: 0,
		},
		{
			name:          "Multiple queries (OR logic)",
			query:         []string{"Meeting", "Urgent"},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := module.filterEventsByQuery(events, tt.query)
			assert.Len(t, results, tt.expectedCount)
		})
	}
}

func TestHandleEventsByDate(t *testing.T) {
	// Mock generic.GetBody
	originalGetBody := generic.GetBody
	defer func() { generic.GetBody = originalGetBody }()

	generic.GetBody = func(url string) ([]byte, error) {
		return mockEventsJSON, nil
	}

	config := Config{
		Sources: []Source{
			{Name: "Test Source", URL: "http://example.com/events.json"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	module := New(config, logger)

	// Test case: Find events in Jan 2026
	params := dateParams{sparkmcp.DateRangeParams{
		StartDate: "2026-01-01",
		EndDate:   "2026-01-31",
	}}

	result, _, err := module.handleEventsByDate(context.Background(), nil, params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	assert.True(t, ok)

	var resultEvents []Event
	err = json.Unmarshal([]byte(textContent.Text), &resultEvents)
	assert.NoError(t, err)
	assert.Len(t, resultEvents, 2) // Should match Jan 10 and Jan 15 events
	assert.Equal(t, "Team Meeting", resultEvents[0].Summary)
	assert.Equal(t, "Doctor Appointment", resultEvents[1].Summary)
}

func TestHandleEventsByQuery(t *testing.T) {
	// Mock generic.GetBody
	originalGetBody := generic.GetBody
	defer func() { generic.GetBody = originalGetBody }()

	generic.GetBody = func(url string) ([]byte, error) {
		return mockEventsJSON, nil
	}

	config := Config{
		Sources: []Source{
			{Name: "Test Source", URL: "http://example.com/events.json"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	module := New(config, logger)

	// Test case: Find "Dinner"
	params := queryParams{
		Query: []string{"Dinner"},
	}

	result, _, err := module.handleEventsByQuery(context.Background(), nil, params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	assert.True(t, ok)

	var resultEvents []Event
	err = json.Unmarshal([]byte(textContent.Text), &resultEvents)
	assert.NoError(t, err)
	assert.Len(t, resultEvents, 1)
	assert.Equal(t, "Dinner", resultEvents[0].Summary)
}

func TestLoadAllEvents_ErrorHandling(t *testing.T) {
	// Mock generic.GetBody to fail
	originalGetBody := generic.GetBody
	defer func() { generic.GetBody = originalGetBody }()

	generic.GetBody = func(url string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	config := Config{
		Sources: []Source{
			{Name: "Bad Source", URL: "http://example.com/bad.json"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	module := New(config, logger)

	events := module.loadAllEvents(config)
	assert.Empty(t, events)
}
