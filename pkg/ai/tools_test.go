package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatToolArgs(t *testing.T) {
	assert.Empty(t, FormatToolArgs(nil))
	assert.Equal(t, `(query: "weather")`, FormatToolArgs(map[string]any{"query": "weather"}))

	longValue := strings.Repeat("x", maxToolArgLengthLog+10)
	assert.Equal(t, `(query: "`+strings.Repeat("x", maxToolArgLengthLog)+`...")`, FormatToolArgs(map[string]any{"query": longValue}))
}
