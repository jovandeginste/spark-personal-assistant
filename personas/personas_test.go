package personas_test

import (
	"strings"
	"testing"

	"github.com/adrg/frontmatter"
	"github.com/jovandeginste/spark-personal-assistant/personas"
	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
)

func TestPersonas(t *testing.T) {
	fs := personas.FS()
	dir, err := fs.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read embedded directory: %v", err)
	}

	for _, entry := range dir {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		if entry.Name() == "ideas.md" {
			continue // Skip non-persona documentation file
		}

		t.Run(entry.Name(), func(t *testing.T) {
			f, err := fs.Open(entry.Name())
			if err != nil {
				t.Fatalf("failed to open file %s: %v", entry.Name(), err)
			}
			defer f.Close()

			var assistantConfig ai.AssistantConfig
			rest, err := frontmatter.Parse(f, &assistantConfig)
			if err != nil {
				t.Errorf("failed to parse frontmatter for %s: %v", entry.Name(), err)
			}

			if assistantConfig.Name == "" {
				t.Errorf("persona %s is missing 'name' in frontmatter", entry.Name())
			}

			if len(rest) == 0 {
				t.Errorf("persona %s has no content body (style description)", entry.Name())
			}
		})
	}
}

func TestButlerPersona(t *testing.T) {
	// Specific test for the default persona 'butler.md' to ensure it's valid and loaded correctly
	fs := personas.FS()
	f, err := fs.Open("butler.md")
	if err != nil {
		t.Fatalf("failed to open butler.md: %v", err)
	}
	defer f.Close()

	var assistantConfig ai.AssistantConfig
	_, err = frontmatter.Parse(f, &assistantConfig)
	if err != nil {
		t.Errorf("failed to parse butler.md frontmatter: %v", err)
	}

	expectedName := "Spark ⚡️"
	if assistantConfig.Name != expectedName {
		t.Errorf("butler.md name mismatch: expected '%s', got '%s'", expectedName, assistantConfig.Name)
	}
}
