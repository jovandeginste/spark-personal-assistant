package ai

import (
	"context"
	"log"
	"strings"

	"github.com/ollama/ollama/api"
)

type ollamaClient struct {
	model string
}

func (c ollamaClient) APIKey() string {
	return ""
}

func (c ollamaClient) Model() string {
	return c.model
}

func promptToOllama(p Prompt, data any) (string, error) {
	prompt, err := p(data)
	if err != nil {
		return "", err
	}

	return strings.Join(prompt, "\n"), nil
}

func (c ollamaClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	prompt, err := promptToOllama(p, data)
	if err != nil {
		return "", err
	}

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	req := &api.GenerateRequest{
		Model:  "gemma3:1b",
		Prompt: prompt,
		Options: map[string]any{
			"num_ctx": 20000,
		},
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
