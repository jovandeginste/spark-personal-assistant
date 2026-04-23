package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Config struct {
	DatabasePath string `mapstructure:"database_path"`
}

type Module struct {
	sparkmcp.BaseModule
	db *gorm.DB
	mu sync.RWMutex
}

type SemanticMemory struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Entity    string    `gorm:"not null;uniqueIndex:idx_entity_fact"`
	Fact      string    `gorm:"not null;uniqueIndex:idx_entity_fact"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (SemanticMemory) TableName() string {
	return "semantic_memory"
}

type EpisodicMemory struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Event     string    `gorm:"not null"`
	Lesson    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (EpisodicMemory) TableName() string {
	return "episodic_memory"
}

type ProceduralMemory struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	Trigger     string    `gorm:"not null;uniqueIndex:idx_trigger_instruction"`
	Instruction string    `gorm:"not null;uniqueIndex:idx_trigger_instruction"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (ProceduralMemory) TableName() string {
	return "procedural_memory"
}

func New(config Config, logger *slog.Logger) *Module {
	return &Module{
		BaseModule: sparkmcp.NewBaseModule(config, logger.With("module", "memory")),
	}
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if config.DatabasePath == "" {
		return errors.New("memory database path is not configured")
	}
	return nil
}

func (m *Module) Initialize() error {
	config := m.Config().(Config)

	db, err := gorm.Open(sqlite.Open(config.DatabasePath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.AutoMigrate(&SemanticMemory{}, &EpisodicMemory{}, &ProceduralMemory{}); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	m.db = db
	return nil
}

func (m *Module) Register(server *mcp.Server) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_semantic_store",
		Description: "Store a semantic memory (factual knowledge about entities, people, or concepts). Use this for facts that are independent of specific experiences.",
	}, m.handleSemanticStore)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_semantic_search",
		Description: "Search semantic memory (factual knowledge) by entity name or concept.",
	}, m.handleSemanticSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_episodic_store",
		Description: "Store an episodic memory (specific past events, interactions, and lessons learned).",
	}, m.handleEpisodicStore)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_episodic_search",
		Description: "Search episodic memory (past experiences and lessons) by keyword or concept.",
	}, m.handleEpisodicSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memory_procedural_store",
		Description: "Store a procedural memory (learned behaviors and instructions that should become automatic).",
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
		return nil, nil, errors.New("entity and fact are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	memory := SemanticMemory{Entity: params.Entity, Fact: params.Fact}
	result := m.db.Where(SemanticMemory{Entity: params.Entity, Fact: params.Fact}).FirstOrCreate(&memory)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to store semantic memory: %w", result.Error)
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
		return nil, nil, errors.New("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var memories []SemanticMemory
	likeQuery := "%" + params.Query + "%"
	result := m.db.Where("entity LIKE ? OR fact LIKE ?", likeQuery, likeQuery).Order("created_at desc").Limit(20).Find(&memories)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to search semantic memory: %w", result.Error)
	}

	var results []string
	for _, memory := range memories {
		results = append(results, fmt.Sprintf("- **%s**: %s", memory.Entity, memory.Fact))
	}

	text := "No semantic memories found matching the query."
	if len(results) > 0 {
		text = "Found the following semantic memories:\n\n" + joinWithNewlines(results)
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
		return nil, nil, errors.New("event and lesson are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	memory := EpisodicMemory{Event: params.Event, Lesson: params.Lesson}
	result := m.db.Create(&memory)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to store episodic memory: %w", result.Error)
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
		return nil, nil, errors.New("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var memories []EpisodicMemory
	likeQuery := "%" + params.Query + "%"
	result := m.db.Where("event LIKE ? OR lesson LIKE ?", likeQuery, likeQuery).Order("created_at desc").Limit(20).Find(&memories)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to search episodic memory: %w", result.Error)
	}

	var results []string
	for _, memory := range memories {
		results = append(results, fmt.Sprintf("- [%s] **Event**: %s\n  **Lesson**: %s", memory.CreatedAt.Format("2006-01-02 15:04:05.999999999Z07:00"), memory.Event, memory.Lesson))
	}

	text := "No episodic memories found matching the query."
	if len(results) > 0 {
		text = "Found the following episodic memories:\n\n" + joinWithNewlines(results)
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
		return nil, nil, errors.New("trigger and instruction are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	memory := ProceduralMemory{Trigger: params.Trigger, Instruction: params.Instruction}
	result := m.db.Where(ProceduralMemory{Trigger: params.Trigger, Instruction: params.Instruction}).FirstOrCreate(&memory)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to store procedural memory: %w", result.Error)
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
		return nil, nil, errors.New("query is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var memories []ProceduralMemory
	likeQuery := "%" + params.Query + "%"
	result := m.db.Where("trigger LIKE ? OR instruction LIKE ?", likeQuery, likeQuery).Order("created_at desc").Limit(20).Find(&memories)
	if result.Error != nil {
		return nil, nil, fmt.Errorf("failed to search procedural memory: %w", result.Error)
	}

	var results []string
	for _, memory := range memories {
		results = append(results, fmt.Sprintf("- **When**: %s\n  **Do**: %s", memory.Trigger, memory.Instruction))
	}

	text := "No procedural memories found matching the query."
	if len(results) > 0 {
		text = "Found the following procedural memories:\n\n" + joinWithNewlines(results)
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
	var resultSb strings.Builder
	for i, v := range s {
		if i > 0 {
			resultSb.WriteString("\n\n")
		}
		resultSb.WriteString(v)
	}
	result += resultSb.String()
	return result
}
