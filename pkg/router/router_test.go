package router

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) APIKey() string                       { return "test" }
func (m *mockAIClient) Model() string                        { return "test" }
func (m *mockAIClient) Logger() *slog.Logger                 { return slog.Default() }
func (m *mockAIClient) GeneratePrompt(ctx context.Context, data any) (string, error) {
	return m.response, m.err
}
func (m *mockAIClient) GenerateWithTools(ctx context.Context, data any, tools []ai.Tool, toolExecutor ai.ToolExecutor, history []string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	for _, tool := range tools {
		if tool.Name == "router_list_destinations" {
			_, _ = toolExecutor(ctx, "router_list_destinations", nil)
		}
		if tool.Name == "router_send_message" {
			_, _ = toolExecutor(ctx, "router_send_message", map[string]any{
				"to":      []any{"room2@matrix"},
				"content": "AI response message",
			})
		}
	}
	return m.response, nil
}
func (m *mockAIClient) UploadFile(ctx context.Context, name string, data []byte, mime string) (string, error) {
	return "", nil
}
func (m *mockAIClient) ListFiles(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockAIClient) DeleteFile(ctx context.Context, name string) error {
	return nil
}

func TestRouter_RegisterAndList(t *testing.T) {
	r := NewRouter(nil, nil)

	addr := Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Matrix Room 1",
	}

	err := r.RegisterAddress(addr)
	require.NoError(t, err)

	fetched, ok := r.GetAddress("room1@matrix")
	assert.True(t, ok)
	assert.Equal(t, "matrix", fetched.System)

	addrs := r.ListAddresses()
	assert.Len(t, addrs, 1)

	// Invalid registration
	err = r.RegisterAddress(Address{ID: "", InstanceName: "room1", System: "matrix"})
	assert.Error(t, err)

	err = r.RegisterAddress(Address{ID: "test@matrix", InstanceName: "", System: "matrix"})
	assert.Error(t, err)

	err = r.RegisterAddress(Address{ID: "wrong@matrix", InstanceName: "room1", System: "matrix"})
	assert.Error(t, err)
}

func TestRouter_SubmitMessage(t *testing.T) {
	ctx := context.Background()
	r := NewRouter(&mockAIClient{response: "Hello back"}, nil)

	var sentMessages []Message
	targetAddr := Address{
		ID:           "room2@matrix",
		InstanceName: "room2",
		System:       "matrix",
		Description:  "Target Matrix Room",
		SendFunc: func(ctx context.Context, msg Message) error {
			sentMessages = append(sentMessages, msg)
			return nil
		},
	}

	sourceAddr := Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Source Matrix Room",
	}

	require.NoError(t, r.RegisterAddress(targetAddr))
	require.NoError(t, r.RegisterAddress(sourceAddr))

	msg := Message{
		Metadata: Metadata{
			To:      []string{"room2@matrix"},
			Subject: "Test Subject",
		},
		Content: "Hello target",
	}

	err := r.SubmitMessage(ctx, "room1@matrix", msg)
	require.NoError(t, err)

	// Verify "from" was set by router
	assert.Len(t, sentMessages, 1)
	assert.Equal(t, "room1@matrix", sentMessages[0].Metadata.From)
	assert.Equal(t, "Hello target", sentMessages[0].Content)
	assert.Equal(t, "Test Subject", sentMessages[0].Metadata.Subject)
}

func TestRouter_RouteToAIAndTools(t *testing.T) {
	ctx := context.Background()
	a := app.NewApp()
	r := NewRouter(&mockAIClient{response: "Processed by AI"}, a)

	targetAddr := Address{
		ID:           "room2@matrix",
		InstanceName: "room2",
		System:       "matrix",
		Description:  "Target Matrix Room",
		SendFunc: func(ctx context.Context, msg Message) error {
			return nil
		},
	}

	sourceAddr := Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Source Matrix Room",
		SendFunc: func(ctx context.Context, msg Message) error {
			return nil
		},
	}

	require.NoError(t, r.RegisterAddress(targetAddr))
	require.NoError(t, r.RegisterAddress(sourceAddr))

	msg := Message{
		Metadata: Metadata{
			To:      []string{"ai"},
			Subject: "AI Question",
		},
		Content: "Help me",
	}

	r.routeToAI(ctx, msg)
}

func TestRouter_UnknownSource(t *testing.T) {
	ctx := context.Background()
	r := NewRouter(nil, nil)

	err := r.SubmitMessage(ctx, "unknown", Message{Content: "test"})
	assert.Error(t, err)
}
