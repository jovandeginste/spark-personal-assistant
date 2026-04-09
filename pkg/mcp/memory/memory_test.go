package memory

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryModule(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "memory.db")

	config := Config{
		DatabasePath: dbPath,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	m := New(config, logger)

	err := m.Initialize()
	require.NoError(t, err)
	defer m.db.Close()

	t.Run("Semantic Memory", func(t *testing.T) {
		// Store
		storeRes, _, err := m.handleSemanticStore(context.Background(), nil, semanticStoreParams{
			Entity: "Alice",
			Fact:   "Alice is the point of contact for API documentation",
		})
		require.NoError(t, err)
		assert.Contains(t, storeRes.Content[0].(*mcp.TextContent).Text, "Alice")

		// Search
		searchRes, _, err := m.handleSemanticSearch(context.Background(), nil, semanticSearchParams{
			Query: "Alice",
		})
		require.NoError(t, err)
		assert.Contains(t, searchRes.Content[0].(*mcp.TextContent).Text, "Alice is the point of contact")
	})

	t.Run("Episodic Memory", func(t *testing.T) {
		// Store
		storeRes, _, err := m.handleEpisodicStore(context.Background(), nil, episodicStoreParams{
			Event:  "Last time this client emailed about deadline extensions, my response was too rigid and created friction",
			Lesson: "Be more accommodating with deadline extensions for this client.",
		})
		require.NoError(t, err)
		assert.Contains(t, storeRes.Content[0].(*mcp.TextContent).Text, "Successfully stored")

		// Search
		searchRes, _, err := m.handleEpisodicSearch(context.Background(), nil, episodicSearchParams{
			Query: "deadline extensions",
		})
		require.NoError(t, err)
		assert.Contains(t, searchRes.Content[0].(*mcp.TextContent).Text, "deadline extensions")
	})

	t.Run("Procedural Memory", func(t *testing.T) {
		// Store
		storeRes, _, err := m.handleProceduralStore(context.Background(), nil, proceduralStoreParams{
			Trigger:     "Always prioritize emails about API documentation",
			Instruction: "Review API documentation emails first and ensure accurate responses.",
		})
		require.NoError(t, err)
		assert.Contains(t, storeRes.Content[0].(*mcp.TextContent).Text, "API documentation")

		// Search
		searchRes, _, err := m.handleProceduralSearch(context.Background(), nil, proceduralSearchParams{
			Query: "API documentation",
		})
		require.NoError(t, err)
		assert.Contains(t, searchRes.Content[0].(*mcp.TextContent).Text, "Review API documentation emails first")
	})
	
	t.Run("Enabled", func(t *testing.T) {
		err := m.Enabled()
		assert.NoError(t, err)
		
		m2 := New(Config{}, logger)
		err = m2.Enabled()
		assert.Error(t, err)
	})
	
	t.Run("Register", func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		err := m.Register(server)
		assert.NoError(t, err)
	})
}
