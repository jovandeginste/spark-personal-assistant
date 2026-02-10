package mcp

import (
	"log/slog"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestBaseModule(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := map[string]string{"foo": "bar"}
	module := NewBaseModule(config, logger)

	t.Run("Initialize", func(t *testing.T) {
		// Test initial state
		assert.Equal(t, logger, module.Logger())
		assert.Equal(t, config, module.Config())
	})

	t.Run("Setters", func(t *testing.T) {
		newLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		newConfig := map[string]string{"baz": "qux"}

		module.SetLogger(newLogger)
		module.SetConfig(newConfig)

		assert.Equal(t, newLogger, module.Logger())
		assert.Equal(t, newConfig, module.Config())
	})

	t.Run("Defaults", func(t *testing.T) {
		// Test default implementations
		assert.NoError(t, module.Initialize())
		assert.NoError(t, module.Enabled())
		assert.NoError(t, module.Register(&mcp.Server{}))
	})
}
