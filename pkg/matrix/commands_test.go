package matrix

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAIClient is a mock of ai.Client interface
type MockAIClient struct {
	mock.Mock
}

func (m *MockAIClient) APIKey() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAIClient) Model() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAIClient) Logger() *slog.Logger {
	args := m.Called()
	return args.Get(0).(*slog.Logger)
}

func (m *MockAIClient) GeneratePrompt(ctx context.Context, data any) (string, error) {
	args := m.Called(ctx, data)
	return args.String(0), args.Error(1)
}

func (m *MockAIClient) GenerateWithTools(ctx context.Context, data any, tools []ai.Tool, toolExecutor ai.ToolExecutor, fileURIs []string) (string, error) {
	args := m.Called(ctx, data, tools, toolExecutor, fileURIs)
	return args.String(0), args.Error(1)
}

func (m *MockAIClient) UploadFile(ctx context.Context, name string, data []byte, mimeType string) (string, error) {
	args := m.Called(ctx, name, data, mimeType)
	return args.String(0), args.Error(1)
}

func (m *MockAIClient) ListFiles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAIClient) DeleteFile(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestParseInput_Prompt_With_Name(t *testing.T) {
	mockAIClient := new(MockAIClient)

	config := app.Config{
		Prompts: map[string]string{
			"weather": "What is the weather like?",
		},
		MCPServers: map[string]app.MCPServerConfig{},
	}

	mc := &MatrixConfig{
		App:      &app.App{Config: config},
		AIData:   &app.AIData{},
		AIClient: mockAIClient,
	}

	// Expect GenerateWithTools to be called
	mockAIClient.On("GenerateWithTools", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("It is sunny", nil)

	res, err := mc.parseInput("!prompt weather", "roomID")
	assert.NoError(t, err)
	assert.Equal(t, "It is sunny", res)

	// Verify AIData.EmployerQuestion was set correctly
	assert.Equal(t, []string{"What is the weather like?"}, mc.AIData.EmployerQuestion)

	mockAIClient.AssertExpectations(t)
}

func TestParseInput_Prompt_Not_Found(t *testing.T) {
	config := app.Config{
		Prompts: map[string]string{
			"weather": "What is the weather like?",
		},
	}

	mc := &MatrixConfig{
		App:    &app.App{Config: config},
		AIData: &app.AIData{},
	}

	res, err := mc.parseInput("!prompt unknown", "roomID")
	assert.NoError(t, err)
	assert.Equal(t, "Prompt 'unknown' not found in configuration", res)
}

func TestParseInput_Prompt_Usage(t *testing.T) {
	mc := &MatrixConfig{}

	res, err := mc.parseInput("!prompt", "roomID")
	assert.NoError(t, err)
	assert.Equal(t, "Usage: !prompt <name>", res)
}

func TestParseInput_Prompt_With_MultiWord_Name(t *testing.T) {
	mockAIClient := new(MockAIClient)

	config := app.Config{
		Prompts: map[string]string{
			"my custom prompt": "This is a custom prompt",
		},
		MCPServers: map[string]app.MCPServerConfig{},
	}

	mc := &MatrixConfig{
		App:      &app.App{Config: config},
		AIData:   &app.AIData{},
		AIClient: mockAIClient,
	}

	// Expect GenerateWithTools to be called
	mockAIClient.On("GenerateWithTools", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("Generated response", nil)

	res, err := mc.parseInput("!prompt my custom prompt", "roomID")
	assert.NoError(t, err)
	assert.Equal(t, "Generated response", res)

	// Verify AIData.EmployerQuestion was set correctly
	assert.Equal(t, []string{"This is a custom prompt"}, mc.AIData.EmployerQuestion)

	mockAIClient.AssertExpectations(t)
}
