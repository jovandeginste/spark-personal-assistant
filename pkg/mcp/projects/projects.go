package projects

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gosimple/slug"
	safe "github.com/jovandeginste/spark-personal-assistant/pkg/helpers/safe"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Config struct {
	Path string `mapstructure:"path"`
}

type Module struct {
	sparkmcp.BaseModule
	// Mutex for basic concurrency safety on file writes. For a larger app, a file-specific lock might be better.
	mu sync.RWMutex
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
			"type":       "object",
			"properties": map[string]any{},
		},
	}, m.handleListProjects)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "projects_summaries",
		Description: "Retrieve summaries (index.md) of all projects",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
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
		Name:        "project_read_file",
		Description: "Read the content of a specific file within a project",
	}, m.handleReadFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_read_file_headers",
		Description: "Read the markdown headers of a specific file within a project. Useful to get an overview of the structure of a file before reading or updating it.",
	}, m.handleReadFileHeaders)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_replace_section",
		Description: "Replace or delete a specific section of a file in a project. Provide the old content (which must match exactly and be unique) and the new content.",
	}, m.handleReplaceSection)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_create_file",
		Description: "Create a new file in an existing project. The file must not exist yet.",
	}, m.handleCreateFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_delete_file",
		Description: "Delete a file from a project",
	}, m.handleDeleteFile)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "project_delete",
		Description: "Delete an entire project",
	}, m.handleDeleteProject)

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

type readFileParams struct {
	Project  string `json:"project" jsonschema:"Name of the project"`
	FileName string `json:"file_name" jsonschema:"Name of the file to read (e.g., notes.md, index.md)"`
}

type readFileHeadersParams struct {
	Project  string `json:"project" jsonschema:"Name of the project"`
	FileName string `json:"file_name" jsonschema:"Name of the file to read (e.g., notes.md, index.md)"`
}

type replaceSectionParams struct {
	Project    string `json:"project" jsonschema:"Name of the project"`
	FileName   string `json:"file_name" jsonschema:"Name of the file to update (e.g., notes.md, index.md)"`
	OldContent string `json:"old_content" jsonschema:"The exact content to replace. Must not be empty. Must be unique within the file."`
	NewContent string `json:"new_content" jsonschema:"The new content to insert. Leave empty to delete the old content."`
}

type createFileParams struct {
	Project  string `json:"project" jsonschema:"Name of the project"`
	FileName string `json:"file_name" jsonschema:"Name of the file to create (e.g., notes.md, index.md)"`
	Content  string `json:"content" jsonschema:"Initial content of the file"`
}

type deleteFileParams struct {
	Project  string `json:"project" jsonschema:"Name of the project"`
	FileName string `json:"file_name" jsonschema:"Name of the file to delete"`
}

type deleteProjectParams struct {
	Name    string `json:"name" jsonschema:"Name of the project to delete"`
	Content string `json:"content" jsonschema:"Content of the index.md file to confirm deletion"`
}

// Handlers

func (m *Module) handleListProjects(ctx context.Context, request *mcp.CallToolRequest, params struct{}) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "listProjects")
	logger.Debug("Listing projects")

	m.mu.RLock()
	defer m.mu.RUnlock()

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

	if len(projects) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No projects found.",
				},
			},
		}, nil, nil
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

	m.mu.RLock()
	defer m.mu.RUnlock()

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

	if summaries.Len() == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No project summaries found.",
				},
			},
		}, nil, nil
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

	m.mu.Lock()
	defer m.mu.Unlock()

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

	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("project '%s' does not exist", safeName)
		}
		return nil, nil, fmt.Errorf("failed to read project directory: %w", err)
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Project: %s\n\n", safeName))

	fileFound := false
	for _, entry := range entries {
		// Read text based files or commonly used formats
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".md") || strings.HasSuffix(entry.Name(), ".txt") || strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".csv")) {
			filePath := filepath.Clean(filepath.Join(projectPath, entry.Name()))
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				logger.Error("Failed to read file", "file", filePath, "error", err)
				contentBuilder.WriteString(fmt.Sprintf("## File: %s\n\n[Error reading file]\n\n---\n\n", entry.Name()))
				continue
			}
			contentBuilder.WriteString(fmt.Sprintf("## File: %s\n\n%s\n\n---\n\n", entry.Name(), string(fileContent)))
			fileFound = true
		} else if !entry.IsDir() {
			// Mention other files exist but don't output content
			contentBuilder.WriteString(fmt.Sprintf("## File: %s\n\n[Content hidden for non-text file]\n\n---\n\n", entry.Name()))
		}
	}

	if !fileFound && contentBuilder.Len() == len(fmt.Sprintf("# Project: %s\n\n", safeName)) {
		contentBuilder.WriteString("No readable text files found in this project.")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: contentBuilder.String(),
			},
		},
	}, nil, nil
}

func (m *Module) handleReadFile(ctx context.Context, request *mcp.CallToolRequest, params readFileParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "readFile")

	if params.Project == "" || params.FileName == "" {
		return nil, nil, errors.New("project and file_name are required")
	}

	safeProject := slug.Make(params.Project)

	ext := filepath.Ext(params.FileName)
	nameWithoutExt := strings.TrimSuffix(params.FileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return nil, nil, errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return nil, nil, errors.New("invalid file path")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeProject)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file '%s' does not exist in project '%s'", safeFile, safeProject)
		}
		logger.Error("Failed to read file", "file", filePath, "error", err)
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(fileContent),
			},
		},
	}, nil, nil
}

func extractHeaders(content []byte) ([]string, error) {
	var headers []string

	md := goldmark.New()
	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader)

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if heading, ok := n.(*ast.Heading); ok {
			level := heading.Level
			prefix := strings.Repeat("#", level)

			var textContent []byte
			for c := heading.FirstChild(); c != nil; c = c.NextSibling() {
				if textNode, ok := c.(*ast.Text); ok {
					textContent = append(textContent, textNode.Segment.Value(content)...)
				}
			}

			headers = append(headers, fmt.Sprintf("%s %s", prefix, string(textContent)))
		}
		return ast.WalkContinue, nil
	})

	return headers, err
}

func (m *Module) handleReadFileHeaders(ctx context.Context, request *mcp.CallToolRequest, params readFileHeadersParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "readFileHeaders")

	if params.Project == "" || params.FileName == "" {
		return nil, nil, errors.New("project and file_name are required")
	}

	safeProject := slug.Make(params.Project)

	ext := filepath.Ext(params.FileName)
	nameWithoutExt := strings.TrimSuffix(params.FileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return nil, nil, errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return nil, nil, errors.New("invalid file path")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeProject)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file '%s' does not exist in project '%s'", safeFile, safeProject)
		}
		logger.Error("Failed to read file", "file", filePath, "error", err)
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	headers, err := extractHeaders(fileContent)
	if err != nil {
		logger.Error("Failed to parse markdown", "file", filePath, "error", err)
		return nil, nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	headerText := strings.Join(headers, "\n")
	if len(headers) == 0 {
		headerText = "No markdown headers found in this file."
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: headerText,
			},
		},
	}, nil, nil
}

func (m *Module) resolveFileAndProject(projectName, fileName string) (string, string, string, string, error) {
	if projectName == "" || fileName == "" {
		return "", "", "", "", errors.New("project and file_name are required")
	}

	config := m.Config().(Config)
	safeProject := slug.Make(projectName)

	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return "", "", "", "", errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return "", "", "", "", errors.New("invalid project path")
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return "", "", "", "", errors.New("invalid file path")
	}

	return safeProject, safeFile, projectPath, filePath, nil
}

func (m *Module) handleReplaceSection(ctx context.Context, request *mcp.CallToolRequest, params replaceSectionParams) (*mcp.CallToolResult, any, error) {
	logger := m.Logger().With("handler", "replaceSection")
	logger.Debug("Replacing section in file", "project", params.Project, "file", params.FileName)

	safeProject, safeFile, projectPath, filePath, err := m.resolveFileAndProject(params.Project, params.FileName)
	if err != nil {
		return nil, nil, err
	}

	if params.OldContent == "" {
		return nil, nil, errors.New("old_content must not be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeProject)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file '%s' does not exist. Use project_create_file to create new files", safeFile)
		}
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(fileBytes)

	// Check if unique
	occurrences := strings.Count(content, params.OldContent)
	if occurrences == 0 {
		return nil, nil, errors.New("old_content not found in file")
	}
	if occurrences > 1 {
		return nil, nil, errors.New("old_content is not unique in file, please provide more context")
	}

	content = strings.Replace(content, params.OldContent, params.NewContent, 1)

	// Write to a temporary file first, then rename for atomicity
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(content), 0o600); err != nil {
		return nil, nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filePath); err != nil {
		// Clean up the temp file if rename fails
		os.Remove(tmpFile)
		return nil, nil, fmt.Errorf("failed to save file atomically: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("File '%s' in project '%s' updated successfully", safeFile, safeProject),
			},
		},
	}, nil, nil
}

func (m *Module) handleCreateFile(ctx context.Context, request *mcp.CallToolRequest, params createFileParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "createFile")
	logger.Debug("Creating file in project", "project", params.Project, "file", params.FileName)

	if params.Project == "" || params.FileName == "" {
		return nil, nil, errors.New("project and file_name are required")
	}

	safeProject := slug.Make(params.Project)

	ext := filepath.Ext(params.FileName)
	nameWithoutExt := strings.TrimSuffix(params.FileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return nil, nil, errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return nil, nil, errors.New("invalid file path")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeProject)
	}

	if _, err := os.Stat(filePath); err == nil {
		return nil, nil, fmt.Errorf("file '%s' already exists in project '%s', use project_replace_section to modify it", safeFile, safeProject)
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to check if file exists: %w", err)
	}

	// Write to a temporary file first, then rename for atomicity
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(params.Content), 0o600); err != nil {
		return nil, nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filePath); err != nil {
		// Clean up the temp file if rename fails
		os.Remove(tmpFile)
		return nil, nil, fmt.Errorf("failed to save file atomically: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("File '%s' created in project '%s' successfully", safeFile, safeProject),
			},
		},
	}, nil, nil
}

func (m *Module) handleDeleteFile(ctx context.Context, request *mcp.CallToolRequest, params deleteFileParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "deleteFile")
	logger.Debug("Deleting file", "project", params.Project, "file", params.FileName)

	if params.Project == "" || params.FileName == "" {
		return nil, nil, errors.New("project and file_name are required")
	}

	safeProject := slug.Make(params.Project)

	ext := filepath.Ext(params.FileName)
	nameWithoutExt := strings.TrimSuffix(params.FileName, ext)
	safeFile := slug.Make(nameWithoutExt) + ext

	if safeFile == "" || safeFile == ext || strings.Contains(safeFile, "/") || strings.Contains(safeFile, "\\") {
		return nil, nil, errors.New("invalid file name")
	}

	projectPath := filepath.Clean(filepath.Join(config.Path, safeProject))
	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	filePath := filepath.Clean(filepath.Join(projectPath, safeFile))
	if err := safe.IsSubPath(projectPath, filePath); err != nil {
		return nil, nil, errors.New("invalid file path")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file '%s' does not exist in project '%s'", safeFile, safeProject)
		}
		return nil, nil, fmt.Errorf("failed to delete file: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("File '%s' deleted from project '%s'", safeFile, safeProject),
			},
		},
	}, nil, nil
}

func (m *Module) handleDeleteProject(ctx context.Context, request *mcp.CallToolRequest, params deleteProjectParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "deleteProject")
	logger.Debug("Deleting project", "name", params.Name)

	if params.Name == "" {
		return nil, nil, errors.New("project name is required")
	}

	if params.Content == "" {
		return nil, nil, errors.New("project content is required for deletion confirmation")
	}

	safeName := slug.Make(params.Name)
	projectPath := filepath.Clean(filepath.Join(config.Path, safeName))

	if err := safe.IsSubPath(config.Path, projectPath); err != nil {
		return nil, nil, errors.New("invalid project path")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("project '%s' does not exist", safeName)
	}

	indexPath := filepath.Clean(filepath.Join(projectPath, "index.md"))
	indexContent, err := os.ReadFile(indexPath)
	if err == nil {
		if strings.TrimSpace(string(indexContent)) != strings.TrimSpace(params.Content) {
			return nil, nil, errors.New("provided content does not match index.md content. Project deletion aborted")
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to read index.md for confirmation: %w", err)
	}

	if err := os.RemoveAll(projectPath); err != nil {
		return nil, nil, fmt.Errorf("failed to delete project: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Project '%s' deleted successfully", safeName),
			},
		},
	}, nil, nil
}
