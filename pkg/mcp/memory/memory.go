package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	_ "github.com/glebarez/go-sqlite"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	DatabasePath string `mapstructure:"database_path"`
}

type Module struct {
	sparkmcp.BaseModule
	db *sql.DB
	mu sync.RWMutex
}

func New(config Config, logger *slog.Logger) *Module {
	return &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "memory")),
	}
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.DatabasePath == "" {
		return fmt.Errorf("memory database path is not configured")
	}
	return nil
}

func (m *Module) Initialize() error {
	config := m.Config().(Config)
	
	db, err := sql.Open("sqlite", config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS semantic_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity TEXT NOT NULL,
			fact TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(entity, fact)
		)`,
		`CREATE TABLE IF NOT EXISTS episodic_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event TEXT NOT NULL,
			lesson TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS procedural_memory (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trigger TEXT NOT NULL,
			instruction TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(trigger, instruction)
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to initialize schema: %w", err)
		}
	}

	m.db = db
	return nil
}

func (m *Module) Register(server *mcp.Server) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_semantic_store",
		Description: "Store a semantic memory (factual knowledge about entities, people, or concepts). Examples: 'Alice is the point of contact for API documentation', 'John prefers morning meetings'. Use this for facts that are independent of specific experiences.",
	}, m.handleSemanticStore)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_semantic_search",
		Description: "Search semantic memory (factual knowledge) by entity name or concept.",
	}, m.handleSemanticSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_episodic_store",
		Description: "Store an episodic memory (specific past events, interactions, and lessons learned). Examples: 'Last time this client emailed about deadline extensions, my response was too rigid', 'When emails contain the phrase quick question, they usually require detailed technical explanations'.",
	}, m.handleEpisodicStore)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_episodic_search",
		Description: "Search episodic memory (past experiences and lessons) by keyword or concept.",
	}, m.handleEpisodicSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_procedural_store",
		Description: "Store a procedural memory (learned behaviors and instructions that should become automatic). Examples: 'Always prioritize emails about API documentation', 'Use a more helpful tone in responses to technical questions'.",
	}, m.handleProceduralStore)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_procedural_search",
		Description: "Search procedural memory (learned behaviors and instructions) by trigger or situation.",
	}, m.handleProceduralSearch)

	return nil
}

type semanticStoreParams struct {
	Entity string `json:"entity" jsonschema:"The subject of the fact (e.g., 'Alice', 'API Documentation', 'Project X')"`
	Fact   string `json:"fact" jsonschema:"The factual information to store about the entity"`
}

type semanticSearchParams struct {
	Query string `json:"query" jsonschema:"The entity or keyword to search for"`
}

type episodicStoreParams struct {
	Event  string `json:"event" jsonschema:"A description of the past event or interaction"`
	Lesson string `json:"lesson" jsonschema:"What was learned or the takeaway from this event"`
}

type episodicSearchParams struct {
	Query string `json:"query" jsonschema:"Keyword or concept to search for in past events and lessons"`
}

type proceduralStoreParams struct {
	Trigger     string `json:"trigger" jsonschema:"The situation, condition, or stimulus that triggers the behavior"`
	Instruction string `json:"instruction" jsonschema:"The rule, process, or behavior to follow when triggered"`
}

type proceduralSearchParams struct {
	Query string `json:"query" jsonschema:"Keyword or situation to search for in instructions"`
}

func (m *Module) handleSemanticStore(ctx context.Context, request *mcp.CallToolRequest, params semanticStoreParams) (*mcp.CallToolResult, any, error) {
	if params.Entity == "" || params.Fact == "" {
		return nil, nil, fmt.Errorf("entity and fact are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("INSERT OR IGNORE INTO semantic_memory (entity, fact) VALUES (?, ?)", params.Entity, params.Fact)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store semantic memory: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully stored semantic memory about '%s'.", params.Entity),
			},
		},
	}, nil, nil
}

func (m *Module) handleSemanticSearch(ctx context.Context, request *mcp.CallToolRequest, params semanticSearchParams) (*mcp.CallToolResult, any, error) {
	if params.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query("SELECT entity, fact FROM semantic_memory WHERE entity LIKE ? OR fact LIKE ? ORDER BY created_at DESC LIMIT 20", 
		"%"+params.Query+"%", "%"+params.Query+"%")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search semantic memory: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var entity, fact string
		if err := rows.Scan(&entity, &fact); err != nil {
			return nil, nil, err
		}
		results = append(results, fmt.Sprintf("- **%s**: %s", entity, fact))
	}

	text := "No semantic memories found matching the query."
	if len(results) > 0 {
		text = "Found the following semantic memories:\n\n" + fmt.Sprintf("%s", joinWithNewlines(results))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}, nil, nil
}

func (m *Module) handleEpisodicStore(ctx context.Context, request *mcp.CallToolRequest, params episodicStoreParams) (*mcp.CallToolResult, any, error) {
	if params.Event == "" || params.Lesson == "" {
		return nil, nil, fmt.Errorf("event and lesson are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("INSERT INTO episodic_memory (event, lesson) VALUES (?, ?)", params.Event, params.Lesson)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store episodic memory: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Successfully stored episodic memory.",
			},
		},
	}, nil, nil
}

func (m *Module) handleEpisodicSearch(ctx context.Context, request *mcp.CallToolRequest, params episodicSearchParams) (*mcp.CallToolResult, any, error) {
	if params.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query("SELECT event, lesson, created_at FROM episodic_memory WHERE event LIKE ? OR lesson LIKE ? ORDER BY created_at DESC LIMIT 20", 
		"%"+params.Query+"%", "%"+params.Query+"%")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search episodic memory: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var event, lesson, createdAt string
		if err := rows.Scan(&event, &lesson, &createdAt); err != nil {
			return nil, nil, err
		}
		results = append(results, fmt.Sprintf("- [%s] **Event**: %s\n  **Lesson**: %s", createdAt, event, lesson))
	}

	text := "No episodic memories found matching the query."
	if len(results) > 0 {
		text = "Found the following episodic memories:\n\n" + fmt.Sprintf("%s", joinWithNewlines(results))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}, nil, nil
}

func (m *Module) handleProceduralStore(ctx context.Context, request *mcp.CallToolRequest, params proceduralStoreParams) (*mcp.CallToolResult, any, error) {
	if params.Trigger == "" || params.Instruction == "" {
		return nil, nil, fmt.Errorf("trigger and instruction are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec("INSERT OR IGNORE INTO procedural_memory (trigger, instruction) VALUES (?, ?)", params.Trigger, params.Instruction)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store procedural memory: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Successfully stored procedural memory for trigger '%s'.", params.Trigger),
			},
		},
	}, nil, nil
}

func (m *Module) handleProceduralSearch(ctx context.Context, request *mcp.CallToolRequest, params proceduralSearchParams) (*mcp.CallToolResult, any, error) {
	if params.Query == "" {
		return nil, nil, fmt.Errorf("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query("SELECT trigger, instruction FROM procedural_memory WHERE trigger LIKE ? OR instruction LIKE ? ORDER BY created_at DESC LIMIT 20", 
		"%"+params.Query+"%", "%"+params.Query+"%")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search procedural memory: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var trigger, instruction string
		if err := rows.Scan(&trigger, &instruction); err != nil {
			return nil, nil, err
		}
		results = append(results, fmt.Sprintf("- **When**: %s\n  **Do**: %s", trigger, instruction))
	}

	text := "No procedural memories found matching the query."
	if len(results) > 0 {
		text = "Found the following procedural memories:\n\n" + fmt.Sprintf("%s", joinWithNewlines(results))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}, nil, nil
}

func joinWithNewlines(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += "\n\n"
		}
		result += v
	}
	return result
}
