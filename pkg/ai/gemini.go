package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"google.golang.org/genai"
)

type Client struct {
	APIKey string

	client *genai.Client
	ctx    context.Context
}

func (c *Client) Init(apiKey string) error {
	c.ctx = context.Background()

	client, err := genai.NewClient(c.ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return err
	}

	c.client = client

	return nil
}

func NewClient() (*Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	c := &Client{}

	if err := c.Init(apiKey); err != nil {
		return nil, err
	}

	return c, nil
}

type Prompt func(data any) ([]*genai.Content, error)

func PromptFor(format string) (Prompt, error) {
	switch format {
	case "today":
		return PromptToday, nil
	case "week":
		return PromptWeek, nil
	case "full":
		return PromptFull, nil
	}

	return nil, fmt.Errorf("unknown format: %s", format)
}

const promptPreamble = "You are a personal assistant named 'Spark'. You provide an overview in Markdown for your employers. Use a polite British style and accent. Use the metric system and 24 hour clock notation. Use emojis. Translate all entries to English. The following entries consist a list of items, for which you should compile a summarized overview of todo's, a schedule and reminders."

func PromptWeek(data any) ([]*genai.Content, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: promptPreamble},
				{Text: "Only include this week's entries."},
				{Text: "Today is: " + time.Now().Format("2006-01-02")},
				{Text: "Information:"},
				{Text: string(j)},
			},
		},
	}

	return c, nil
}

func PromptToday(data any) ([]*genai.Content, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: promptPreamble},
				{Text: "Start your response with a suitable greeting and comment about today's weather if you have this information. Only include today's and tomorrow's entries. Be verbose."},
				{Text: "Today is: " + time.Now().Format("2006-01-02")},
				{Text: "Information:"},
				{Text: string(j)},
			},
		},
	}

	return c, nil
}

func PromptFull(data any) ([]*genai.Content, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: promptPreamble},
				{Text: "Add a quick summary of the past week's important entries. Be verbose about today's entries. Add a quick summary of future important entries - one line per day. Add weather information for days with outside entries."},
				{Text: "Today is: " + time.Now().Format("2006-01-02")},
				{Text: "Information:"},
				{Text: string(j)},
			},
		},
	}

	return c, nil
}

func GeneratePrompt(p Prompt, data any) (string, error) {
	ai, err := NewClient()
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{}
	model := "models/gemini-2.5-flash-preview-04-17"
	// model := "gemini-2.5-pro-exp-03-25"

	slog.Info("Generating summary...")

	content, err := p(data)
	if err != nil {
		return "", err
	}

	result, err := ai.client.Models.GenerateContent(ai.ctx, model, content, config)
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
