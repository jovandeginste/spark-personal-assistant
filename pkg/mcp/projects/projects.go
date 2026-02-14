package projects

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gosimple/slug"
	safe "github.com/jovandeginste/spark-personal-assistant/pkg/helpers/safe"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type Module struct {
	sparkmcp.BaseModule
}

func New(config Config, logger *slog.Logger) *Module {
	return &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "projects")),
	}
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.Path == "" {
		return errors.New("projects path is not configured")
	}
	// Verify directory exists or create it
	if err := os.MkdirAll(filepath.Clean(config.Path), 0o755); err != nil {
		return fmt.Errorf("failed to create projects directory: %w", err)
	}
	return nil
}

func (m *Module) Register(server *mcp.Server) error {
	m.Logger().Info("Registering Projects MCP tools")

	mcp.AddTool(server, &mcp.Tool{
		Name:        "projects_list",
		Description: "List all existing projects",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"_dummy": map[string]any{"type": "string"},
			},
		},
	}, m.handleListProjects)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "projects_summaries",
		Description: "Retrieve summaries (index.md) of all projects",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"_dummy": map[string]any{"type": "string"},
			},
		},
	}, m.handleProjectSummaries)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_create",
		Description: "Create a new project with a synopsis",
	}, m.handleCreateProject)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_get",
		Description: "Retrieve all information (files) of a single project",
	}, m.handleGetProject)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_update",
		Description: "Add or update a file in a project",
	}, m.handleUpdateProject)

	return nil
}

// Params definitions

type createProjectParams struct {
	Name     string `json:"name" jsonschema:"Name of the project (will be the directory name)"`
	Synopsis string `json:"synopsis" jsonschema:"Initial synopsis/content for index.md"`
}

type getProjectParams struct {
	Name string `json:"name" jsonschema:"Name of the project to retrieve"`
}

type updateProjectParams struct {
	Project  string `json:"project" jsonschema:"Name of the project"`
	FileName string `json:"file_name" jsonschema:"Name of the file to add/update (e.g., notes.md, index.md)"`
	Content  string `json:"content" jsonschema:"Content to write to the file"`
}

// Handlers

func (m *Module) handleListProjects(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "listProjects")
	logger.Debug("Listing projects")

	entries, err := os.ReadDir(filepath.Clean(config.Path))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list projects: %w", err)
	}

	projects := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			projects = append(projects, entry.Name())
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: strings.Join(projects, "\n"),
			},
		},
	}, nil, nil
}

func (m *Module) handleProjectSummaries(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "projectSummaries")
	logger.Debug("Listing project summaries")

	entries, err := os.ReadDir(filepath.Clean(config.Path))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var summaries strings.Builder
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			indexPath := filepath.Clean(filepath.Join(config.Path, entry.Name(), "index.md"))

			if err := safe.IsSubPath(config.Path, indexPath); err != nil {
				continue
			}

			content, err := os.ReadFile(indexPath)
			if err == nil {
				summaries.WriteString(fmt.Sprintf("Project: %s\n%s\n---\n", entry.Name(), string(content)))
			} else {
				summaries.WriteString(fmt.Sprintf("Project: %s\n(No index.md found)\n---\n", entry.Name()))
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: summaries.String(),
			},
		},
	}, nil, nil
}

func (m *Module) handleCreateProject(ctx context.Context, request *mcp.CallToolRequest, params createProjectParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "createProject")
	logger.Debug("Creating project", "name", params.Name)

	if params.Name == "" {
		return nil, nil, errors.New("project name is required")
	}

	safeName := slug.Make(params.Name)
	if safeName == "" {
		return nil, nil, errors.New("invalid project name (resulting slug is empty)")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeName))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' already exists", safeName)
	}

	if err := os.Mkdir(projectPath, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	indexPath := filepath.Clean(filepath.Join(projectPath, "index.md"))
	if err := os.WriteFile(indexPath, []byte(params.Synopsis), 0o600); err != nil {
		return nil, nil, fmt.Errorf("failed to write index.md: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Project '%s' created successfully", safeName),
			},
		},
	}, nil, nil
}

func (m *Module) handleGetProject(ctx context.Context, request *mcp.CallToolRequest, params getProjectParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "getProject")

	if params.Name == "" {
		return nil, nil, errors.New("project name is required")
	}

	safeName := slug.Make(params.Name)
	projectPath := filepath.Clean(filepath.Join(config.Path, safeName))

	// Security check to ensure we stay within config.Path
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("project '%s' does not exist", safeName)
		}
		return nil, nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Project: %s\n\n", safeName))

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			filePath := filepath.Clean(filepath.Join(projectPath, entry.Name()))
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				logger.Error("Failed to read file", "file", filePath, "error", err)
				continue
			}
			contentBuilder.WriteString(fmt.Sprintf("## File: %s\n\n%s\n\n---\n\n", entry.Name(), string(fileContent)))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: contentBuilder.String(),
			},
		},
	}, nil, nil
}

func (m *Module) handleUpdateProject(ctx context.Context, request *mcp.CallToolRequest, params updateProjectParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "updateProject")
	logger.Debug("Updating project", "project", params.Project, "file", params.FileName)

	if params.Project == "" || params.FileName == "" {
		return nil, nil, errors.New("project and file_name are required")
	}

	safeProject := slug.Make(params.Project)

	// We want to keep the extension, so we split, slugify the name, and rejoin
	ext := filepath.Ext(params.FileName)
	nameWithoutExt := strings.TrimSuffix(params.FileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	// Additional check to prevent directory traversal via extension manipulation or empty names
	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return nil, nil, errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeProject)
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return nil, nil, errors.New("invalid file path")
	}

	if err := os.WriteFile(filePath, []byte(params.Content), 0o600); err != nil {
		return nil, nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("File '%s' in project '%s' updated successfully", safeFile, safeProject),
			},
		},
	}, nil, nil
}
