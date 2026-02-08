package diary

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a temporary directory for tests
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "diary_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

func TestIsValidUser(t *testing.T) {
	users := []string{"Alice", "Bob"}

	assert.True(t, isValidUser("Alice", users))
	assert.True(t, isValidUser("alice", users)) // Case insensitive
	assert.True(t, isValidUser("Bob", users))
	assert.False(t, isValidUser("Charlie", users))
	assert.False(t, isValidUser("", users))

	// No users configured = disallow all
	assert.False(t, isValidUser("Any", []string{}))
}

func TestAddDiaryEntry(t *testing.T) {
	dir := setupTestDir(t)
	config := Config{
		Path:  dir,
		Users: []string{"TestUser"},
	}

	logger := slog.Default()
	m := New(config, logger)
	ctx := context.Background()

	// 1. Success case
	_, _, err := m.handleAddEntry(ctx, nil, addEntryParams{
		User:  "TestUser",
		Entry: "Hello Diary",
		Date:  "2023-10-27",
	})
	require.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(filepath.Join(dir, "TestUser", "2023-10-27.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Hello Diary")

	// 2. Invalid User
	_, _, err = m.handleAddEntry(ctx, nil, addEntryParams{
		User:  "UnknownUser",
		Entry: "Should fail",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user")

	// 3. Default Date
	_, _, err = m.handleAddEntry(ctx, nil, addEntryParams{
		User:  "TestUser",
		Entry: "Today's entry",
	})
	require.NoError(t, err)
	today := time.Now().Format("2006-01-02")
	content, err = os.ReadFile(filepath.Join(dir, "TestUser", today+".md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "Today's entry")
}

func TestReadDiary(t *testing.T) {
	dir := setupTestDir(t)
	config := Config{
		Path:  dir,
		Users: []string{"TestUser"},
	}
	logger := slog.Default()

	// Pre-populate data
	userDir := filepath.Join(dir, "TestUser")
	require.NoError(t, os.MkdirAll(userDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "2023-10-01.md"), []byte("Entry 1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "2023-10-05.md"), []byte("Entry 2"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "2023-10-10.md"), []byte("Entry 3"), 0o600))

	m := New(config, logger)
	ctx := context.Background()

	// 1. Read all
	res, _, err := m.handleReadDiary(ctx, nil, readParams{User: "TestUser"})
	require.NoError(t, err)
	text := res.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, text, "Entry 1")
	assert.Contains(t, text, "Entry 2")
	assert.Contains(t, text, "Entry 3")

	// 2. Date Range
	res, _, err = m.handleReadDiary(ctx, nil, readParams{
		User:      "TestUser",
		StartDate: "2023-10-02",
		EndDate:   "2023-10-08",
	})
	require.NoError(t, err)
	text = res.Content[0].(*mcp.TextContent).Text
	assert.NotContains(t, text, "Entry 1")
	assert.Contains(t, text, "Entry 2")
	assert.NotContains(t, text, "Entry 3")

	// 3. Invalid User
	_, _, err = m.handleReadDiary(ctx, nil, readParams{User: "Unknown"})
	require.Error(t, err)
}

func TestUpdateDiaryEntry(t *testing.T) {
	dir := setupTestDir(t)
	config := Config{
		Path:  dir,
		Users: []string{"TestUser"},
	}

	// Pre-populate
	userDir := filepath.Join(dir, "TestUser")
	require.NoError(t, os.MkdirAll(userDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "2023-10-01.md"), []byte("Old Content"), 0o600))

	logger := slog.Default()
	m := New(config, logger)
	ctx := context.Background()

	// 1. Success Update
	_, _, err := m.handleUpdateEntry(ctx, nil, updateParams{
		User:  "TestUser",
		Date:  "2023-10-01",
		Entry: "New Content",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(userDir, "2023-10-01.md"))
	require.NoError(t, err)
	assert.Equal(t, "New Content\n", string(content))

	// 2. Not Found
	_, _, err = m.handleUpdateEntry(ctx, nil, updateParams{
		User:  "TestUser",
		Date:  "2023-10-02", // Doesn't exist
		Entry: "Content",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry not found")
}

func TestDeleteDiaryEntry(t *testing.T) {
	dir := setupTestDir(t)
	config := Config{
		Path:  dir,
		Users: []string{"TestUser"},
	}

	// Pre-populate
	userDir := filepath.Join(dir, "TestUser")
	require.NoError(t, os.MkdirAll(userDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "2023-10-01.md"), []byte("Content"), 0o600))

	logger := slog.Default()
	m := New(config, logger)
	ctx := context.Background()

	// 1. Success Delete
	_, _, err := m.handleDeleteEntry(ctx, nil, deleteParams{
		User: "TestUser",
		Date: "2023-10-01",
	})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(userDir, "2023-10-01.md"))
	assert.True(t, os.IsNotExist(err))

	// 2. Not Found
	_, _, err = m.handleDeleteEntry(ctx, nil, deleteParams{
		User: "TestUser",
		Date: "2023-10-01", // Already deleted
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entry not found")
}

func TestListUsers(t *testing.T) {
	config := Config{
		Path:  "/tmp",
		Users: []string{"Alice", "Bob"},
	}

	m := New(config, slog.Default())
	ctx := context.Background()

	res, _, err := m.handleListUsers(ctx, nil, listUsersParams{})
	require.NoError(t, err)

	text := res.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, text, "Alice")
	assert.Contains(t, text, "Bob")
}

func TestListUsers_Empty(t *testing.T) {
	config := Config{
		Path:  "/tmp",
		Users: []string{},
	}

	m := New(config, slog.Default())
	ctx := context.Background()

	res, _, err := m.handleListUsers(ctx, nil, listUsersParams{})
	require.NoError(t, err)

	text := res.Content[0].(*mcp.TextContent).Text
	assert.Contains(t, text, "No users configured")
}
