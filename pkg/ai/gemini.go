package ai

import (
	"context"

	"google.golang.org/genai"
)

type geminiClient struct {
	apiKey    string
	model     string
	assistant AssistantConfig
}

func (c geminiClient) APIKey() string {
	return c.apiKey
}

func (c geminiClient) Model() string {
	return c.model
}

func (c geminiClient) convertPrompt(p Prompt, data any) (*genai.Content, error) {
	prompt, err := p(c.assistant, data)
	if err != nil {
		return nil, err
	}

	var parts []*genai.Part

	for _, part := range prompt {
		parts = append(parts, &genai.Part{Text: part})
	}

	return genai.NewContentFromParts(parts, genai.RoleUser), nil
}

func (c geminiClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return "", err
	}

	prompt, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{}

	result, err := client.Models.GenerateContent(ctx, c.model, []*genai.Content{prompt}, config)
	if err != nil {
		return "", err
	}

	for _, c := range result.Candidates {
		if len(c.Content.Parts) == 0 {
			continue
		}

		for _, part := range c.Content.Parts {
			return part.Text, nil
		}
	}

	return "", nil
}
