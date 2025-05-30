package ai

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/genai"
)

type geminiClient struct {
	apiKey    string
	model     string
	ttsModel  string
	ttsVoice  string
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

	parts := make([]*genai.Part, 0, len(prompt))

	for _, part := range prompt {
		parts = append(parts, &genai.Part{Text: part})
	}

	return genai.NewContentFromParts(parts, genai.RoleUser), nil
}

func (c geminiClient) GeneratePrompt(ctx context.Context, p Prompt, data any) (string, error) {
	prompt, err := c.convertPrompt(p, data)
	if err != nil {
		return "", err
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{}

	for i := range 10 {
		result, err := client.Models.GenerateContent(ctx, c.model, []*genai.Content{prompt}, config)
		if err != nil {
			if i < 10 {
				fmt.Println("Retrying in 30 seconds...")
				time.Sleep(30 * time.Second)
				continue
			}

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
	}

	return "", nil
}

func (c geminiClient) GenerateSpeech(ctx context.Context, text string) ([]byte, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey})
	if err != nil {
		return nil, err
	}

	config := &genai.GenerateContentConfig{
		ResponseModalities: []string{"AUDIO"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: c.ttsVoice,
				},
			},
		},
	}

	result, err := client.Models.GenerateContent(
		ctx,
		c.ttsModel,
		[]*genai.Content{genai.NewContentFromText(text, genai.RoleUser)},
		config,
	)
	if err != nil {
		return nil, err
	}

	for _, c := range result.Candidates {
		if len(c.Content.Parts) == 0 {
			continue
		}

		for _, part := range c.Content.Parts {
			return part.InlineData.Data, nil
		}
	}

	return nil, nil
}
