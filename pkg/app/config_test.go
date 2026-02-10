package app_test

import (
	"os"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestPersonaLoading(t *testing.T) {
	// Setup a temporary config file
	f, err := os.CreateTemp("", "spark-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	// Initialize app with this config
	a := app.App{ConfigFile: f.Name()}

	// Test default behavior (no config, should fallback to default persona 'butler')
	// Note: We are testing SetDefaults logic mostly here
	a.SetDefaults()

	assert.Equal(t, "Spark ⚡️", a.Config.Assistant.Name)
	assert.NotEmpty(t, a.Config.Assistant.Style)
	assert.Contains(t, a.Config.Assistant.Style, "highly professional English butler")

	// Test SetPersona manual override
	a.SetPersona("spock")
	assert.Equal(t, "Spock", a.Config.Assistant.Name)
	assert.Contains(t, a.Config.Assistant.Style, "Vulcan")
}
