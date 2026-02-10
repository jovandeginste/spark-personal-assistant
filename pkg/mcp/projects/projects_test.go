package projects

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestModule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "projects_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := Config{
		Path: tempDir,
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	m := New(config, logger)

	t.Run("Initialize and Enable", func(t *testing.T) {
		err := m.Initialize()
		assert.NoError(t, err)

		err = m.Enabled()
		assert.NoError(t, err)

		// Verify directory exists
		info, err := os.Stat(tempDir)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("Create Project", func(t *testing.T) {
		params := createProjectParams{
			Name:     "Test Project", // Should be slugified to test-project
			Synopsis: "A test project synopsis",
		}

		res, _, err := m.handleCreateProject(nil, nil, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "'test-project' created successfully")

		// Verify on disk
		projectPath := filepath.Join(tempDir, "test-project")
		info, err := os.Stat(projectPath)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())

		indexPath := filepath.Join(projectPath, "index.md")
		content, err := os.ReadFile(indexPath)
		assert.NoError(t, err)
		assert.Equal(t, "A test project synopsis", string(content))
	})

	t.Run("List Projects", func(t *testing.T) {
		res, _, err := m.handleListProjects(nil, nil, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "test-project")
	})

	t.Run("Project Summaries", func(t *testing.T) {
		res, _, err := m.handleProjectSummaries(nil, nil, struct{}{})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		text := res.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, text, "Project: test-project")
		assert.Contains(t, text, "A test project synopsis")
	})

	t.Run("Get Project", func(t *testing.T) {
		params := getProjectParams{
			Name: "test-project",
		}
		res, _, err := m.handleGetProject(nil, nil, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		text := res.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, text, "# Project: test-project")
		assert.Contains(t, text, "## File: index.md")
		assert.Contains(t, text, "A test project synopsis")
	})

	t.Run("Update Project", func(t *testing.T) {
		params := updateProjectParams{
			Project:  "test-project",
			FileName: "My Notes.md", // Should be slugified to my-notes.md
			Content:  "Some project notes",
		}

		res, _, err := m.handleUpdateProject(nil, nil, params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "'my-notes.md' in project 'test-project' updated successfully")

		// Verify on disk
		notesPath := filepath.Join(tempDir, "test-project", "my-notes.md")
		content, err := os.ReadFile(notesPath)
		assert.NoError(t, err)
		assert.Equal(t, "Some project notes", string(content))

		// Check if GetProject picks it up
		getParams := getProjectParams{Name: "test-project"}
		res, _, err = m.handleGetProject(nil, nil, getParams)
		assert.NoError(t, err)
		text := res.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, text, "## File: my-notes.md")
		assert.Contains(t, text, "Some project notes")
	})
	
	t.Run("Security Checks", func(t *testing.T) {
		// Attempt to escape root
		params := createProjectParams{
			Name:     "../escaped", // Should be slugified to escaped
			Synopsis: "should succeed but in safe location",
		}
		res, _, err := m.handleCreateProject(nil, nil, params)
		assert.NoError(t, err)
		assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "'escaped' created successfully")

		// Verify it was created in the right place
		safePath := filepath.Join(tempDir, "escaped")
		info, err := os.Stat(safePath)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())

		// Attempt to write outside project
		updateParams := updateProjectParams{
			Project:  "test-project",
			FileName: "../hack.md", // Should be slugified to hack.md
			Content:  "should succeed as hack.md",
		}
		res, _, err = m.handleUpdateProject(nil, nil, updateParams)
		assert.NoError(t, err)
		assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "'hack.md' in project 'test-project' updated successfully")
	})
}
