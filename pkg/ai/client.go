package ai

import (
	"context"
	"fmt"
	"log/slog"
)

type AIConfig struct {
	Type   string `mapstructure:"type"`
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

type AssistantConfig struct {
	Name      string `mapstructure:"name"`
	Style     string `mapstructure:"style"`
	StyleFile string `mapstructure:"style_file"`
}

type Client interface {
	APIKey() string
	Model() string
	GeneratePrompt(context.Context, Prompt, any) (string, error)
}

func NewClient(cc *AIConfig, ac AssistantConfig) (Client, error) {
	var c Client

	switch cc.Type {
	case "gemini":
		c = geminiClient{apiKey: cc.APIKey, model: cc.Model, assistant: ac}
	case "openai":
		c = openaiClient{apiKey: cc.APIKey, model: cc.Model, assistant: ac}
	case "ollama":
		slog.Info("ollama does not work yet - input size is too large?")
		c = ollamaClient{model: cc.Model, assistant: ac}
	default:
		return nil, fmt.Errorf("unknown type: %s", cc.Type)
	}

	return c, nil
}
