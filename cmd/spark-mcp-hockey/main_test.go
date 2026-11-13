package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigEnvOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Chdir(t.TempDir())

	t.Setenv("HOCKEY_PORT", ":9997")

	config, err := loadConfig()
	require.NoError(t, err)
	assert.Equal(t, ":9997", config.Port)
}
