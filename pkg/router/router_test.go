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

func (m *mockAIClient) APIKey() string       { return "test" }
func (m *mockAIClient) Model() string        { return "test" }
func (m *mockAIClient) Logger() *slog.Logger { return slog.Default() }
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

func TestRouter_UnknownTargetReturnsError(t *testing.T) {
	ctx := context.Background()
	r := NewRouter(nil, nil)

	sourceAddr := Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Source Matrix Room",
	}
	require.NoError(t, r.RegisterAddress(sourceAddr))

	err := r.SubmitMessage(ctx, "room1@matrix", Message{
		Metadata: Metadata{To: []string{"mail:jo@dwarfy.be"}},
		Content:  "hello",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown target address: mail:jo@dwarfy.be")
}

func TestRouter_AIOnlyTargetAllowed(t *testing.T) {
	ctx := context.Background()
	r := NewRouter(nil, nil)

	sourceAddr := Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Source Matrix Room",
	}
	require.NoError(t, r.RegisterAddress(sourceAddr))

	err := r.SubmitMessage(ctx, "room1@matrix", Message{
		Metadata: Metadata{To: []string{"ai"}},
		Content:  "hello ai",
	})
	require.NoError(t, err)
}

func TestRouter_ThreadedToolNoticeForMatrixSource(t *testing.T) {
	ctx := context.Background()
	a := app.NewApp()
	a.Config.Matrix = map[string]app.MatrixConfig{"room1": {ThreadedTools: true}}
	r := NewRouter(&mockAIClient{response: "Processed by AI"}, a)

	var notices []Message
	require.NoError(t, r.RegisterAddress(Address{
		ID:           "room1@matrix",
		InstanceName: "room1",
		System:       "matrix",
		Description:  "Source Matrix Room",
		SendFunc: func(ctx context.Context, msg Message) error {
			notices = append(notices, msg)
			return nil
		},
	}))
	require.NoError(t, r.RegisterAddress(Address{
		ID:           "room2@matrix",
		InstanceName: "room2",
		System:       "matrix",
		Description:  "Target Matrix Room",
	}))

	r.routeToAI(ctx, Message{
		Metadata: Metadata{
			From:    "room1@matrix",
			To:      []string{"ai"},
			Subject: "AI Question",
			Extra: map[string]any{
				"room_id":  "!room:id",
				"event_id": "$event",
			},
		},
		Content: "Help me",
	})

	require.NotEmpty(t, notices)
	assert.Equal(t, "notice", notices[0].Metadata.Extra["msgtype"])
	assert.Equal(t, "$event", notices[0].Metadata.Extra["event_id"])
	assert.Contains(t, notices[0].Content, "> Using tool: `router_list_destinations`")
}
