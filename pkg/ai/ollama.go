package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/ollama/ollama/api"
)

type ollamaClient struct {
	model     string
	assistant AssistantConfig
}

func (c ollamaClient) APIKey() string {
	return ""
}

func (c ollamaClient) Model() string {
	return c.model
}

func (c ollamaClient) convertPrompt(p Prompt, data any) (string, error) {
	prompt, err := p(c.assistant, data)
	if err != nil {
		return "", err
	}

	return strings.Join(prompt, "\n"), nil
}

func (c ollamaClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	prompt, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		return "", fmt.Errorf("failed to create ollama client from environment: %w", err)
	}

	req := &api.GenerateRequest{
		Model:  c.Model(),
		Prompt: prompt,
		// Options can be added here if needed, potentially from config
	}

	var result string

	respFunc := func(resp api.GenerateResponse) error {
		// Only print the response here; GenerateResponse has a number of other
		// interesting fields you want to examine.
		result = resp.Response
		return nil
	}

	if err := client.Generate(ctx, req, respFunc); err != nil {
		return "", err
	}

	return result, nil
}
