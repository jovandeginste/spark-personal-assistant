package ai

import (
	"context"
	"fmt"
	"log/slog"
)

type AIConfig struct {
	Type      string `mapstructure:"type"`
	APIKey    string `mapstructure:"api_key"`
	Model     string `mapstructure:"model"`
	Assistant struct {
		Name  string `mapstructure:"name"`
		Style string `mapstructure:"style"`
	} `mapstructure:"assistant"`
}

type Client interface {
	APIKey() string
	Model() string
	GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error)
}

func NewClient(cc *AIConfig) (Client, error) {
	switch cc.Type {
	case "gemini":
		return geminiClient{apiKey: cc.APIKey, model: cc.Model}, nil
	case "openai":
		return openaiClient{apiKey: cc.APIKey, model: cc.Model}, nil
	case "ollama":
		slog.Info("ollama does not work yet - input size is too large?")
		return ollamaClient{model: cc.Model}, nil
	}

	return nil, fmt.Errorf("unknown type: %s", cc.Type)
}
