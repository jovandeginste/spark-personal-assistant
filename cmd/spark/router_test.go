package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingAIClient struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingAIClient) APIKey() string { return "" }

func (c *blockingAIClient) Model() string { return "test-model" }

func (c *blockingAIClient) Logger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func (c *blockingAIClient) GeneratePrompt(context.Context, any) (string, error) {
	return "", nil
}

func (c *blockingAIClient) UploadFile(context.Context, string, []byte, string) (string, error) {
	return "", nil
}

func (c *blockingAIClient) GenerateWithTools(ctx context.Context, _ any, _ []ai.Tool, _ ai.ToolExecutor, _ []string) (string, error) {
	close(c.started)

	<-c.release

	if err := ctx.Err(); err != nil {
		return "", err
	}

	return "answer", nil
}

func TestSubmitWebPromptSurvivesClientDisconnect(t *testing.T) {
	client := &blockingAIClient{started: make(chan struct{}), release: make(chan struct{})}
	r := router.NewRouter(client, app.NewApp())

	require.NoError(t, r.RegisterAddress(router.Address{
		ID:           "default@web",
		InstanceName: "default",
		System:       "web",
		SendFunc:     func(context.Context, router.Message) error { return nil },
	}))

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	t.Cleanup(cancelRequest)

	req := httptest.NewRequest(http.MethodGet, "/prompt?prompt=hi", nil).WithContext(requestCtx)
	rec := httptest.NewRecorder()
	ec := echo.New().NewContext(req, rec)

	done := make(chan error, 1)

	go func() {
		done <- submitWebPrompt(ec, r, "default@web", "Web Prompt", "hi")
	}()

	<-client.started
	cancelRequest()
	close(client.release)

	require.NoError(t, <-done)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "answer", rec.Body.String())
}
