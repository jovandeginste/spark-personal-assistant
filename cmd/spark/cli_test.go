package main

import (
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCLI(t *testing.T) {
	c := NewCLI(app.NewApp())

	require.NotNil(t, c.rootCmd)

	commands := make(map[string]bool)
	for _, command := range c.rootCmd.Commands() {
		commands[command.Name()] = true
	}

	for _, name := range []string{"print", "chat", "matrix", "router"} {
		assert.True(t, commands[name], "missing command %q", name)
	}
}
