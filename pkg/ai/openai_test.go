package ai

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGenerateWithToolsHandlesEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[]}`)
	}))
	defer server.Close()

	c := openaiClient{
		apiKey:    "test-key",
		model:     "test-model",
		baseURL:   server.URL + "/",
		assistant: &AssistantConfig{Name: "TestAssistant"},
		logger:    slog.Default(),
	}

	_, err := c.GenerateWithTools(context.Background(), nil, []Tool{{
		Name:        "test",
		InputSchema: map[string]any{"type": "object"},
	}}, func(context.Context, string, map[string]any) (string, error) {
		return "", nil
	}, nil)

	require.Error(t, err)
}

func TestGenerateWithToolsSkipsInvalidSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[]}`)
	}))
	defer server.Close()

	c := openaiClient{
		apiKey:    "test-key",
		model:     "test-model",
		baseURL:   server.URL + "/",
		assistant: &AssistantConfig{Name: "TestAssistant"},
		logger:    slog.Default(),
	}

	_, err := c.GenerateWithTools(context.Background(), nil, []Tool{{
		Name:        "test",
		InputSchema: "not-a-schema",
	}}, func(context.Context, string, map[string]any) (string, error) {
		return "", nil
	}, nil)

	require.Error(t, err)
}

func TestUploadFileImage(t *testing.T) {
	c := openaiClient{logger: slog.Default()}

	uri, err := c.UploadFile(context.Background(), "image.png", []byte("hello"), "image/png")
	require.NoError(t, err)
	assert.Equal(t, "data:image/png;base64,aGVsbG8=", uri)
}

func TestUploadFileRejectsNonImage(t *testing.T) {
	c := openaiClient{logger: slog.Default()}

	_, err := c.UploadFile(context.Background(), "document.pdf", []byte("%PDF"), "application/pdf")
	require.ErrorContains(t, err, "only support image attachments")
}

func TestUploadFileRejectsEmptyContent(t *testing.T) {
	c := openaiClient{logger: slog.Default()}

	_, err := c.UploadFile(context.Background(), "image.png", nil, "image/png")
	require.ErrorContains(t, err, "empty file")
}

func TestGenerateWithToolsSendsImages(t *testing.T) {
	var requestBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		requestBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 0,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "seen"},
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
	}

	result, err := c.GenerateWithTools(context.Background(), nil, nil, func(context.Context, string, map[string]any) (string, error) {
		return "", nil
	}, []string{"data:image/png;base64,aGVsbG8="})
	require.NoError(t, err)
	assert.Equal(t, "seen", result)
	assert.True(t, strings.Contains(requestBody, `"image_url"`))
	assert.True(t, strings.Contains(requestBody, "data:image/png;base64,aGVsbG8="))
}
