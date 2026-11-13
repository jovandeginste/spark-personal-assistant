package web

import (
	"fmt"
	"strings"
)

// PromptPayload is the JSON body accepted by the prompt endpoints.
type PromptPayload struct {
	Title   string         `json:"title"`
	Message string         `json:"message"`
	Source  string         `json:"source"`
	Extra   map[string]any `json:"extra"`
}

// Format builds the prompt text from the payload.
func (p PromptPayload) Format() string {
	var prompt strings.Builder

	prompt.WriteString(p.Title + "\n\n" + p.Message)
	if p.Source != "" {
		prompt.WriteString("\n\nSource: " + p.Source)
	}

	if len(p.Extra) > 0 {
		prompt.WriteString("\n\nExtra Metadata:\n")

		for k, v := range p.Extra {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
		}
	}

	return prompt.String()
}
