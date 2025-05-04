package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

type Prompt func(assistant AssistantConfig, data any) ([]string, error)

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

var promptPreamble = []string{
	"Your entire response should be formatted in Markdown",
	"Use the metric system and 24 hour clock notation.",
	"Use conversational style.",
	"Use emojis.",
	"Translate all entries to English.",
	"The following entries consist a list of items.",
	"Today is: " + time.Now().Format("Monday, 2006-01-02"),
}

func (a AssistantConfig) PromptPreamble() []string {
	prompt := []string{
		fmt.Sprintf("Your name is %s.", a.Name),
		fmt.Sprintf("Use the following style: %s.", a.Style),
	}

	return append(prompt, promptPreamble...)
}

func PromptWeek(assistant AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Only include this week's entries.",
			"Compile a schedule and a summarized overview of todo's, and reminders.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptToday(assistant AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Start your response with a suitable greeting and comment about today's weather forecast if you have this information. Only include today's and tomorrow's entries. Be verbose.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptFull(assistant AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Add a quick summary of the past week's important entries. Be verbose about today's entries. Add a quick summary of future important entries - one line per day. Add weather information for days with outside entries.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}
