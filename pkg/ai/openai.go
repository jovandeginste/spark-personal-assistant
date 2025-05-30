package ai

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openaiClient struct {
	apiKey    string
	model     string
	ttsModel  string
	ttsVoice  string
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

	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(prompt))

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

	for i := range 10 {
		result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{prompt},
			Model:    c.Model(),
		})
		if err != nil {
			if i < 10 {
				fmt.Println("Retrying in 30 seconds...")
				time.Sleep(30 * time.Second)
				continue
			}

			return "", err
		}

		for _, c := range result.Choices {
			if len(c.Message.Content) == 0 {
				continue
			}

			return c.Message.Content, nil
		}
	}

	return "", nil
}

func (c openaiClient) GenerateSpeech(ctx context.Context, text string) ([]byte, error) {
	client := openai.NewClient(
		option.WithAPIKey(c.APIKey()),
	)

	result, err := client.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
		Model:          c.ttsModel,
		Voice:          openai.AudioSpeechNewParamsVoice(c.ttsVoice),
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatPCM,
		Instructions:   openai.String(c.assistant.Style),
		Input:          text,
	})
	if err != nil {
		return nil, err
	}

	return io.ReadAll(result.Body)
}
