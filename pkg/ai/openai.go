package ai

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openaiClient struct {
	apiKey    string
	model     string
	assistant AssistantConfig
}

func (c openaiClient) APIKey() string {
	return c.apiKey
}

func (c openaiClient) Model() string {
	return c.model
}

func (c openaiClient) convertPrompt(p Prompt, data any) (openai.ChatCompletionMessageParamUnion, error) {
	prompt, err := p(c.assistant, data)
	if err != nil {
		return openai.ChatCompletionMessageParamUnion{}, err
	}

	var parts []openai.ChatCompletionContentPartUnionParam

	for _, part := range prompt {
		parts = append(parts, openai.TextContentPart(part))
	}

	return openai.UserMessage(parts), nil
}

func (c openaiClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	prompt, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	client := openai.NewClient(
		option.WithAPIKey(c.APIKey()),
	)

	result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{prompt},
		Model:    c.Model(),
	})
	if err != nil {
		return "", err
	}

	for _, c := range result.Choices {
		if len(c.Message.Content) == 0 {
			continue
		}

		return c.Message.Content, nil
	}

	return "", nil
}
