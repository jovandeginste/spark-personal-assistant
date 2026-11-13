package mcpserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	configFile := filepath.Join(".", "test-config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("port: \":8080\"\nweather:\n  apiurl: \"http://file-weather\"\n"), 0o600))

	t.Setenv("MCPSERVER_PORT", ":9999")

	var config struct {
		Port    string `mapstructure:"port"`
		Weather struct {
			APIURL string `mapstructure:"apiurl"`
		} `mapstructure:"weather"`
	}

	err := LoadConfig(ConfigOptions{
		Name:      "test-config",
		EnvPrefix: "MCPSERVER",
		Defaults: map[string]any{
			"weather.apiurl": "http://default-weather",
		},
		Target: &config,
	})
	require.NoError(t, err)

	assert.Equal(t, ":9999", config.Port)
	assert.Equal(t, "http://file-weather", config.Weather.APIURL)
}
