package ai

import (
	"context"
	"fmt"
	"log/slog"
)

type AIConfig struct {
	Type     string `mapstructure:"type"`
	APIKey   string `mapstructure:"api_key"`
	Model    string `mapstructure:"model"`
	TTSModel string `mapstructure:"tts_model"`
	TTSVoice string `mapstructure:"tts_voice"`
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
	GenerateSpeech(context.Context, string) ([]byte, error)
	Logger() *slog.Logger
}

func NewClient(cc *AIConfig, ac AssistantConfig, l *slog.Logger) (Client, error) {
	var c Client

	switch cc.Type {
	case "gemini":
		c = geminiClient{
			apiKey:    cc.APIKey,
			model:     cc.Model,
			ttsModel:  cc.TTSModel,
			ttsVoice:  cc.TTSVoice,
			assistant: ac,
			logger:    l.With("ai_backend", "gemini"),
		}
	case "openai":
		c = openaiClient{
			apiKey:    cc.APIKey,
			model:     cc.Model,
			ttsModel:  cc.TTSModel,
			ttsVoice:  cc.TTSVoice,
			assistant: ac,
			logger:    l.With("ai_backend", "openai"),
		}
	case "ollama":
		l.Warn("ollama does not work yet - input size is too large?")
		c = ollamaClient{
			model:     cc.Model,
			assistant: ac,
			logger:    l.With("ai_backend", "ollama"),
		}
	default:
		return nil, fmt.Errorf("unknown type: %s", cc.Type)
	}

	return c, nil
}
