package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

type openaiClient struct {
	apiKey    string
	model     string
	baseURL   string
	assistant *AssistantConfig
	logger    *slog.Logger
}

func (c openaiClient) Logger() *slog.Logger {
	return c.logger
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

	c.Logger().Info("prompt size", "size", len(fmt.Sprintf("%+v", prompt)))

	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(prompt))

	for _, part := range prompt {
		parts = append(parts, openai.TextContentPart(part))
	}

	return openai.UserMessage(parts), nil
}

func (c openaiClient) GeneratePrompt(ctx context.Context, data any) (string, error) {
	c.Logger().Info("Fetching result from AI...")

	prompt, err := c.convertPrompt(PromptCustom, data)
	if err != nil {
		return "", err
	}

	client := c.newOpenAIClient()

	for i := range 10 {
		result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{prompt},
			Model:    c.Model(),
		})
		if err != nil {
			if i < 10 {
				c.Logger().Warn("Retrying in 30 seconds...", "error", err)
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

func (c openaiClient) GenerateWithTools(ctx context.Context, data any, tools []Tool, executor ToolExecutor, fileURIs []string) (string, error) {
	c.Logger().Info("Fetching result from AI with tools...")

	promptMsg, err := c.convertPrompt(PromptCustom, data)
	if err != nil {
		return "", err
	}

	client := c.newOpenAIClient()

	openaiTools := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		openaiTools = append(openaiTools, openai.ChatCompletionToolParam{
			Type: constant.Function("function"),
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  shared.FunctionParameters(t.InputSchema.(map[string]any)),
			},
		})
	}

	messages := []openai.ChatCompletionMessageParamUnion{promptMsg}

	for i := range MaxToolCalls {
		result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: messages,
			Model:    c.Model(),
			Tools:    openaiTools,
		})
		if err != nil {
			c.Logger().Warn("Error generating content", "error", err, "attempt", i)
			time.Sleep(2 * time.Second)
			continue
		}

		choice := result.Choices[0]
		messages = append(messages, choice.Message.ToParam())

		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		for _, tc := range choice.Message.ToolCalls {
			c.Logger().Info("Tool call received", "tool", tc.Function.Name)

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				c.Logger().Error("Failed to unmarshal tool arguments", "error", err)

				args = make(map[string]any)
			}

			resp, err := executor(ctx, tc.Function.Name, args)
			if err != nil {
				c.Logger().Error("Tool execution failed", "error", err)
				resp = fmt.Errorf("tool execution failed: %w", err).Error()
			}

			messages = append(messages, openai.ToolMessage(resp, tc.ID))
		}
	}

	return "", errors.New("reached maximum iterations with tool calls")
}

func (c openaiClient) UploadFile(ctx context.Context, name string, content []byte, mimeType string) (string, error) {
	return "", errors.New("not implemented")
}

func (c openaiClient) ListFiles(ctx context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (c openaiClient) DeleteFile(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func (c openaiClient) newOpenAIClient() openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(c.APIKey()),
	}

	if c.baseURL != "" {
		opts = append(opts, option.WithBaseURL(c.baseURL))
	}

	return openai.NewClient(opts...)
}
