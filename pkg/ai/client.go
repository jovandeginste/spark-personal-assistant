package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

const MaxToolCalls = 30

type AIConfig struct {
	Type    string `mapstructure:"type"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

type AssistantConfig struct {
	Name      string `mapstructure:"name"`
	Style     string `mapstructure:"style"`
	StyleFile string `mapstructure:"style_file"`
	Language  string `mapstructure:"language"`
	File      string `mapstructure:"file"`
}

type Client interface {
	APIKey() string
	Model() string
	GeneratePrompt(context.Context, Prompt, any) (string, error)
	GenerateWithTools(context.Context, Prompt, any, []Tool, ToolExecutor, []string) (string, error)
	Logger() *slog.Logger
	UploadFile(context.Context, string, []byte, string) (string, error)
	ListFiles(context.Context) ([]string, error)
	DeleteFile(context.Context, string) error
}

type Tool struct {
	Name        string
	Description string
	InputSchema any
}

type ToolExecutor func(context.Context, string, map[string]any) (string, error)

func NewClient(cc *AIConfig, ac *AssistantConfig, l *slog.Logger) (Client, error) {
	var c Client

	if cc == nil {
		return nil, errors.New("AI config is nil")
	}

	l = l.With("ai_backend", cc.Type).With("model", cc.Model).With("name", ac.Name)

	switch cc.Type {
	case "gemini":
		c = &geminiClient{
			apiKey:    cc.APIKey,
			model:     cc.Model,
			assistant: ac,
			logger:    l,
		}
	case "openai":
		c = &openaiClient{
			apiKey:    cc.APIKey,
			model:     cc.Model,
			baseURL:   cc.BaseURL,
			assistant: ac,
			logger:    l,
		}
	case "ollama":
		baseURL := cc.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1/"
		}
		c = &openaiClient{
			apiKey:    cc.APIKey,
			model:     cc.Model,
			baseURL:   baseURL,
			assistant: ac,
			logger:    l,
		}
	default:
		return nil, fmt.Errorf("unknown type: %s", cc.Type)
	}

	return c, nil
}
