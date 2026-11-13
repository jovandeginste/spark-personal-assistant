package ai

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClientSendsOpenCodeSessionHeader(t *testing.T) {
	var receivedSession string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSession = r.Header.Get("x-opencode-session")

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 0,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}]
		}`)
	}))
	defer server.Close()

	c := openaiClient{
		apiKey:    "test-key",
		model:     "test-model",
		baseURL:   server.URL + "/",
		assistant: &AssistantConfig{Name: "TestAssistant"},
		logger:    slog.Default(),
		sessionID: "ses_test",
	}

	result, err := c.GeneratePrompt(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
	assert.Equal(t, "ses_test", receivedSession)
}

func TestGeneratePromptReturnsErrorAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not now", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := openaiClient{
		apiKey:    "test-key",
		model:     "test-model",
		baseURL:   server.URL + "/",
		assistant: &AssistantConfig{Name: "TestAssistant"},
		logger:    slog.Default(),
	}

	_, err := c.GeneratePrompt(context.Background(), nil)
	require.Error(t, err)
}
