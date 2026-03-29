package reminders

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
	tempDir, err := os.MkdirTemp("", "reminders_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	remindersFile := filepath.Join(tempDir, "reminders.json")

	config := Config{
		File: remindersFile,
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
		_, err = os.Stat(remindersFile)
		assert.NoError(t, err)
	})

	t.Run("Register", func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, &mcp.ServerOptions{})
		err := m.Register(server)
		assert.NoError(t, err)
	})

	t.Run("List Reminders (Empty)", func(t *testing.T) {
		res, _, err := m.handleListReminders(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No reminders found.", textContent.Text)
	})

	t.Run("Create Invalid Reminder", func(t *testing.T) {
		res, _, err := m.handleCreateReminder(ctx, &req, createReminderParams{ID: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'id'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "test", Name: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'name'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "test", Name: "Test", Message: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'message'")
		assert.Nil(t, res)

		res, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "test", Name: "Test", Message: "Msg", Cron: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'cron'")
		assert.Nil(t, res)
	})

	t.Run("Delete Invalid Reminder", func(t *testing.T) {
		res, _, err := m.handleDeleteReminder(ctx, &req, deleteReminderParams{ID: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid 'id'")
		assert.Nil(t, res)
	})

	t.Run("Create Reminder", func(t *testing.T) {
		params := createReminderParams{
			ID:      "test-1",
			Name:    "Test Reminder",
			Message: "Time's up!",
			Cron:    "* * * * *",
			OneTime: false,
		}

		res, _, err := m.handleCreateReminder(ctx, &req, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "created successfully")
	})

	t.Run("Create Duplicate Reminder", func(t *testing.T) {
		params := createReminderParams{
			ID:      "test-1",
			Name:    "Test Reminder 2",
			Message: "Time's up 2!",
			Cron:    "0 * * * *",
			OneTime: false,
		}

		res, _, err := m.handleCreateReminder(ctx, &req, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, res)
	})

	t.Run("List Reminders (1 Reminder)", func(t *testing.T) {
		res, _, err := m.handleListReminders(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var reminders map[string]Reminder
		err = json.Unmarshal([]byte(textContent.Text), &reminders)
		require.NoError(t, err)

		assert.Len(t, reminders, 1)
		assert.Equal(t, "Test Reminder", reminders["test-1"].Name)
	})

	t.Run("Delete Reminder", func(t *testing.T) {
		params := deleteReminderParams{
			ID: "test-1",
		}

		res, _, err := m.handleDeleteReminder(ctx, &req, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "deleted successfully")
	})

	t.Run("Delete Non-existent Reminder", func(t *testing.T) {
		params := deleteReminderParams{
			ID: "test-1",
		}

		res, _, err := m.handleDeleteReminder(ctx, &req, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, res)
	})

	t.Run("Reminder Parsing", func(t *testing.T) {
		// Clean start
		err = m.writeReminders(map[string]Reminder{})
		require.NoError(t, err)

		_, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "cron-1", Name: "Cron 1", Message: "Msg", Cron: "* * * * *", OneTime: false})
		require.NoError(t, err)

		_, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "cron-2", Name: "Cron 2", Message: "Msg", Cron: "@hourly", OneTime: false})
		require.NoError(t, err)

		futureTime := "2099-01-01T00:00:00Z"
		_, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "rfc-future", Name: "RFC Future", Message: "Msg", Cron: futureTime, OneTime: true})
		require.NoError(t, err)

		pastTime := "2000-01-01T00:00:00Z"
		_, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "rfc-past", Name: "RFC Past", Message: "Msg", Cron: pastTime, OneTime: true})
		require.NoError(t, err)

		_, _, err = m.handleCreateReminder(ctx, &req, createReminderParams{ID: "invalid", Name: "Invalid", Message: "Msg", Cron: "not a valid schedule", OneTime: false})
		require.NoError(t, err)

		res, _, err := m.handleListReminders(ctx, &req, struct{}{})
		require.NoError(t, err)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		var reminders map[string]Reminder
		err = json.Unmarshal([]byte(textContent.Text), &reminders)
		require.NoError(t, err)

		assert.Len(t, reminders, 5)

		assert.NotEmpty(t, reminders["cron-1"].NextTime)
		assert.NotEqual(t, "Past", reminders["cron-1"].NextTime)
		assert.NotEqual(t, "Invalid schedule", reminders["cron-1"].NextTime)

		assert.NotEmpty(t, reminders["cron-2"].NextTime)
		assert.NotEqual(t, "Past", reminders["cron-2"].NextTime)
		assert.NotEqual(t, "Invalid schedule", reminders["cron-2"].NextTime)

		assert.Equal(t, futureTime, reminders["rfc-future"].NextTime)

		assert.Equal(t, "Past", reminders["rfc-past"].NextTime)

		assert.Equal(t, "Invalid schedule", reminders["invalid"].NextTime)

		// Clean up
		err = m.writeReminders(map[string]Reminder{})
		require.NoError(t, err)
	})

	t.Run("List Reminders (Empty Again)", func(t *testing.T) {
		res, _, err := m.handleListReminders(ctx, &req, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Len(t, res.Content, 1)

		textContent, ok := res.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "No reminders found.", textContent.Text)
	})
}

func TestShouldTriggerReminder(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "reminders_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := Config{
		File: filepath.Join(tempDir, "reminders.json"),
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	parser := cron.MustNewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	tests := []struct {
		name     string
		reminder Reminder
		now      time.Time
		expected bool
	}{
		{
			name: "Cron exact match",
			reminder: Reminder{
				Cron: "10 10 10 10 *",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: true,
		},
		{
			name: "Cron no match",
			reminder: Reminder{
				Cron: "11 10 10 10 *",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
		{
			name: "RFC3339 past",
			reminder: Reminder{
				Cron: time.Date(2023, 10, 10, 10, 9, 0, 0, time.UTC).Format(time.RFC3339),
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: true,
		},
		{
			name: "RFC3339 future",
			reminder: Reminder{
				Cron: time.Date(2023, 10, 10, 10, 11, 0, 0, time.UTC).Format(time.RFC3339),
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
		{
			name: "Invalid format",
			reminder: Reminder{
				Cron: "invalid_format",
			},
			now:      time.Date(2023, 10, 10, 10, 10, 30, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := m.shouldTriggerReminder(tc.reminder, parser, tc.now)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestProcessRemindersAndDelete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "reminders_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	remindersFile := filepath.Join(tempDir, "reminders.json")
	config := Config{
		File:     remindersFile,
		Callback: "http://example.com/callback?name=_NAME_",
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	// Create initial reminders
	reminders := map[string]Reminder{
		"past": {
			ID:      "past",
			Name:    "Past Reminder",
			Cron:    time.Now().Add(-1 * time.Minute).Format(time.RFC3339),
			OneTime: true,
		},
		"future": {
			ID:      "future",
			Name:    "Future Reminder",
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
	err = m.writeReminders(reminders)
	require.NoError(t, err)

	// Test processing
	m.processReminders(config, logger)

	// Read reminders back to see if "past" and "onetime_cron" were deleted
	updatedReminders, err := m.readReminders()
	require.NoError(t, err)

	assert.NotContains(t, updatedReminders, "past")
	assert.NotContains(t, updatedReminders, "onetime_cron")
	assert.Contains(t, updatedReminders, "future")
}

func TestDeleteReminders(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "reminders_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	remindersFile := filepath.Join(tempDir, "reminders.json")
	config := Config{
		File: remindersFile,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	m := New(config, logger)

	reminders := map[string]Reminder{
		"1": {ID: "1"},
		"2": {ID: "2"},
	}
	err = m.writeReminders(reminders)
	require.NoError(t, err)

	m.deleteReminders(remindersFile, []string{"1"})

	updatedReminders, err := m.readReminders()
	require.NoError(t, err)

	assert.NotContains(t, updatedReminders, "1")
	assert.Contains(t, updatedReminders, "2")
}
