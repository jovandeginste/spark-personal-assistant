package app

import (
	"encoding/json"
	"log/slog"
	"time"

	"google.golang.org/genai"
)

type EmployerData struct {
	Names []string `mapstructure:"names"`
}

func (a *App) generatePrompt(data any) ([]*genai.Content, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	e, err := json.Marshal(a.Config.EmployerData)
	if err != nil {
		return nil, err
	}

	c := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: "You are a personal assistant named 'Spark'. You provide a daily update in Markdown for your employers. Use a polite British style and accent. Use the metric system and 24 hour clock notation. Start your response with a suitable greeting and comment about today's weather if you have this information. Use emojis. Add a quick summary of the past week's important entries. Be verbose about today's entries. Add a quick summary of future important entries - one line per day. Add weather information for days with outside entries. Translate all entries to English."},
				{Text: "The following entries consist a list of items in the near future or recent past, for which you should compile a summarized overview of todo's, a schedule and reminders."},
				{Text: "Today is: " + time.Now().Format("2006-01-02")},
				{Text: "Employer information and preferences:"},
				{Text: string(e)},
				{Text: "Logbook:"},
				{Text: string(j)},
			},
		},
	}

	return c, nil
}

func (a *App) GeneratePrompt(data any) (string, error) {
	config := &genai.GenerateContentConfig{}
	model := "models/gemini-2.5-flash-preview-04-17"
	// model := "gemini-2.5-pro-exp-03-25"

	slog.Info("Generating summary...")

	content, err := a.generatePrompt(data)
	if err != nil {
		return "", err
	}

	result, err := a.ai.client.Models.GenerateContent(a.ai.ctx, model, content, config)
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
