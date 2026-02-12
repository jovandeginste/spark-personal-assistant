package ai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No enum",
			input:    `{"type": "string"}`,
			expected: `{"type":"string"}`,
		},
		{
			name:     "String enum",
			input:    `{"type": "string", "enum": ["a", "b"]}`,
			expected: `{"enum":["a","b"],"type":"string"}`,
		},
		{
			name:     "Integer enum",
			input:    `{"type": "integer", "enum": [1, 2]}`,
			expected: `{"enum":["1","2"],"type":"string"}`,
		},
		{
			name:     "Mixed enum (should handle it, though rare)",
			input:    `{"type": "string", "enum": [1, "b"]}`,
			expected: `{"enum":["1","b"],"type":"string"}`,
		},
		{
			name:     "Nested object with integer enum",
			input:    `{"type": "object", "properties": {"days": {"type": "integer", "enum": [1, 7]}}}`,
			expected: `{"properties":{"days":{"enum":["1","7"],"type":"string"}},"type":"object"}`,
		},
		{
			name:     "Array with items having integer enum",
			input:    `{"type": "array", "items": [{"type": "integer", "enum": [10, 20]}]}`,
			expected: `{"items":[{"enum":["10","20"],"type":"string"}],"type":"array"}`,
		},
		{
			name:     "Array type definition (nullable string)",
			input:    `{"type": ["string", "null"]}`,
			expected: `{"type":"string"}`,
		},
		{
			name:     "Array type definition (nullable string reversed)",
			input:    `{"type": ["null", "string"]}`,
			expected: `{"type":"string"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawSchema any

			err := json.Unmarshal([]byte(tt.input), &rawSchema)
			assert.NoError(t, err)

			sanitizeSchema(rawSchema)

			outputBytes, err := json.Marshal(rawSchema)
			assert.NoError(t, err)

			// Compare JSON strings. Note: key order might vary, but for simple cases and these inputs it's usually deterministic enough or we could unmarshal again to compare deep equal.
			// Better: Unmarshal expected and actual and compare structure.
			var expectedObj, actualObj any
			json.Unmarshal([]byte(tt.expected), &expectedObj)
			json.Unmarshal(outputBytes, &actualObj)

			assert.Equal(t, expectedObj, actualObj)
		})
	}
}
