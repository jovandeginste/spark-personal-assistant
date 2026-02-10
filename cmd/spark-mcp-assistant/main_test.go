package main

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/stretchr/testify/assert"
)

func TestAllModules(t *testing.T) {
	config := &Config{
		Port: ":8081",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cacheService, _ := caching.NewService(os.TempDir(), time.Hour)

	modules := allModules(config, logger, cacheService)

	assert.NotEmpty(t, modules)
	assert.Len(t, modules, 6)
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := os.MkdirTemp("", "spark-mcp-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configContent := `
port: ":9999"
weather:
  apiurl: "http://test-weather.com"
`
	err = os.WriteFile(tmpDir+"/mcp-config.yaml", []byte(configContent), 0644)
	assert.NoError(t, err)

	// Change working directory to temp dir so viper finds the config
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tmpDir)

	config, err := loadConfig()
	assert.NoError(t, err)
	assert.Equal(t, ":9999", config.Port)
	assert.Equal(t, "http://test-weather.com", config.Weather.APIURL)
}
