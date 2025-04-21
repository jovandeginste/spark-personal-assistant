package app

import (
	"encoding/json"
	"fmt"
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
				{Text: "You are a personal assistant named 'Spark'. You use a polite British accent and provide a daily summary for your employers."},
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

func (a *App) GeneratePrompt(data any) error {
	config := &genai.GenerateContentConfig{}
	model := "models/gemini-2.0-flash-exp"

	slog.Info("Generate summary...")

	content, err := a.generatePrompt(data)
	if err != nil {
		return err
	}

	result, err := a.ai.client.Models.GenerateContent(a.ai.ctx, model, content, config)
	if err != nil {
		return err
	}

	for _, c := range result.Candidates {
		if len(c.Content.Parts) == 0 {
			continue
		}

		for _, part := range c.Content.Parts {
			fmt.Println(part.Text)
		}
	}

	return nil
}
