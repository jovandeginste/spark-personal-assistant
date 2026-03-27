package timers

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/netresearch/go-cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "timers_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	timersFile := filepath.Join(tempDir, "timers.json")

	config := Config{
		File: timersFile,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	ctx := context.Background()
	req := mcp.CallToolRequest{}

	t.Run("Initialize and Enable", func(t *testing.T) {
		err := m.Initialize()
		assert.NoError(t, err)

		err = m.Enabled()
		assert.NoError(t, err)

		// Check if the initial file was created
		_, err = os.Stat(timersFile)
		assert.NoError(t, err)
	})

	t.Run("Register", func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, &mcp.ServerOptions{})
		err := m.Register(server)
		assert.NoError(t, err)
	})

	t.Run("List Timers (Empty)", func(t *testing.T) {
		res, _, err := m.handleListTimers(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No timers found.", textContent.Text)
	})

	t.Run("Create Invalid Timer", func(t *testing.T) {
		res, _, err := m.handleCreateTimer(ctx, &req, createTimerParams{ID: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'id'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "test", Name: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'name'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "test", Name: "Test", Message: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'message'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "test", Name: "Test", Message: "Msg", Cron: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'cron'")
		assert.Nil(t, res)
	})

	t.Run("Delete Invalid Timer", func(t *testing.T) {
		res, _, err := m.handleDeleteTimer(ctx, &req, deleteTimerParams{ID: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'id'")
		assert.Nil(t, res)
	})

	t.Run("Create Timer", func(t *testing.T) {
		params := createTimerParams{
			ID:      "test-1",
			Name:    "Test Timer",
			Message: "Time's up!",
			Cron:    "* * * * *",
			OneTime: false,
		}

		res, _, err := m.handleCreateTimer(ctx, &req, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "created successfully")
	})

	t.Run("Create Duplicate Timer", func(t *testing.T) {
		params := createTimerParams{
			ID:      "test-1",
			Name:    "Test Timer 2",
			Message: "Time's up 2!",
			Cron:    "0 * * * *",
			OneTime: false,
		}

		res, _, err := m.handleCreateTimer(ctx, &req, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, res)
	})

	t.Run("List Timers (1 Timer)", func(t *testing.T) {
		res, _, err := m.handleListTimers(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var timers map[string]Timer
		err = json.Unmarshal([]byte(textContent.Text), &timers)
		require.NoError(t, err)

		assert.Len(t, timers, 1)
		assert.Equal(t, "Test Timer", timers["test-1"].Name)
	})

	t.Run("Delete Timer", func(t *testing.T) {
		params := deleteTimerParams{
			ID: "test-1",
		}

		res, _, err := m.handleDeleteTimer(ctx, &req, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "deleted successfully")
	})

	t.Run("Delete Non-existent Timer", func(t *testing.T) {
		params := deleteTimerParams{
			ID: "test-1",
		}

		res, _, err := m.handleDeleteTimer(ctx, &req, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, res)
	})

	t.Run("Timer Parsing", func(t *testing.T) {
		// Clean start
		err = m.writeTimers(map[string]Timer{})
		require.NoError(t, err)

		_, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "cron-1", Name: "Cron 1", Message: "Msg", Cron: "* * * * *", OneTime: false})
		require.NoError(t, err)

		_, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "cron-2", Name: "Cron 2", Message: "Msg", Cron: "@hourly", OneTime: false})
		require.NoError(t, err)

		futureTime := "2099-01-01T00:00:00Z"
		_, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "rfc-future", Name: "RFC Future", Message: "Msg", Cron: futureTime, OneTime: true})
		require.NoError(t, err)

		pastTime := "2000-01-01T00:00:00Z"
		_, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "rfc-past", Name: "RFC Past", Message: "Msg", Cron: pastTime, OneTime: true})
		require.NoError(t, err)

		_, _, err = m.handleCreateTimer(ctx, &req, createTimerParams{ID: "invalid", Name: "Invalid", Message: "Msg", Cron: "not a valid schedule", OneTime: false})
		require.NoError(t, err)

		res, _, err := m.handleListTimers(ctx, &req, struct{}{})
		require.NoError(t, err)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var timers map[string]Timer
		err = json.Unmarshal([]byte(textContent.Text), &timers)
		require.NoError(t, err)

		assert.Len(t, timers, 5)

		assert.NotEmpty(t, timers["cron-1"].NextTime)
		assert.NotEqual(t, "Past", timers["cron-1"].NextTime)
		assert.NotEqual(t, "Invalid schedule", timers["cron-1"].NextTime)

		assert.NotEmpty(t, timers["cron-2"].NextTime)
		assert.NotEqual(t, "Past", timers["cron-2"].NextTime)
		assert.NotEqual(t, "Invalid schedule", timers["cron-2"].NextTime)

		assert.Equal(t, futureTime, timers["rfc-future"].NextTime)

		assert.Equal(t, "Past", timers["rfc-past"].NextTime)

		assert.Equal(t, "Invalid schedule", timers["invalid"].NextTime)

		// Clean up
		err = m.writeTimers(map[string]Timer{})
		require.NoError(t, err)
	})

	t.Run("List Timers (Empty Again)", func(t *testing.T) {
		res, _, err := m.handleListTimers(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No timers found.", textContent.Text)
	})
}

func TestShouldTriggerTimer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "timers_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := Config{
		File: filepath.Join(tempDir, "timers.json"),
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	tests := []struct {
		name     string
		timer    Timer
		now      time.Time
		expected bool
	}{
		{
			name: "Cron exact match",
			timer: Timer{
				Cron: "10 10 10 10 *",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: true,
		},
		{
			name: "Cron no match",
			timer: Timer{
				Cron: "11 10 10 10 *",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
		{
			name: "RFC3339 past",
			timer: Timer{
				Cron: time.Date(2023, 10, 10, 10, 9, 0, 0, time.UTC).Format(time.RFC3339),
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: true,
		},
		{
			name: "RFC3339 future",
			timer: Timer{
				Cron: time.Date(2023, 10, 10, 10, 11, 0, 0, time.UTC).Format(time.RFC3339),
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
		{
			name: "Invalid format",
			timer: Timer{
				Cron: "invalid_format",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := m.shouldTriggerTimer(tc.timer, parser, tc.now)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProcessTimersAndDelete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "timers_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	timersFile := filepath.Join(tempDir, "timers.json")
	config := Config{
		File:     timersFile,
		Callback: "http://example.com/callback?name=_NAME_",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	// Create initial timers
	timers := map[string]Timer{
		"past": {
			ID:      "past",
			Name:    "Past Timer",
			Cron:    time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			OneTime: true,
		},
		"future": {
			ID:      "future",
			Name:    "Future Timer",
			Cron:    time.Now().Add(10 * time.Minute).Format(time.RFC3339),
			OneTime: true,
		},
		"onetime_cron": {
			ID:      "onetime_cron",
			Name:    "One Time Cron",
			Cron:    "* * * * *",
			OneTime: true,
		},
	}
	err = m.writeTimers(timers)
	require.NoError(t, err)

	// Test processing
	m.processTimers(config, logger)

	// Read timers back to see if "past" and "onetime_cron" were deleted
	updatedTimers, err := m.readTimers()
	require.NoError(t, err)

	assert.NotContains(t, updatedTimers, "past")
	assert.NotContains(t, updatedTimers, "onetime_cron")
	assert.Contains(t, updatedTimers, "future")
}

func TestDeleteTimers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "timers_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	timersFile := filepath.Join(tempDir, "timers.json")
	config := Config{
		File: timersFile,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	timers := map[string]Timer{
		"1": {ID: "1"},
		"2": {ID: "2"},
	}
	err = m.writeTimers(timers)
	require.NoError(t, err)

	m.deleteTimers(timersFile, []string{"1"})

	updatedTimers, err := m.readTimers()
	require.NoError(t, err)

	assert.NotContains(t, updatedTimers, "1")
	assert.Contains(t, updatedTimers, "2")
}
