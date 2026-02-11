package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

type Prompt func(assistant *AssistantConfig, data any) ([]string, error)

func PromptFor(format string) (Prompt, error) {
	switch format {
	case "custom":
		return PromptCustom, nil
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
	"The names in the user data are your employers' names",
	"Use your tools to collect all relevant information.",
}

func (a AssistantConfig) PromptPreamble() []string {
	prompt := []string{
		"Your name is: " + a.Name,
		"Use the following style: " + a.Style,
		"Today is: " + time.Now().Format("Monday, 2006-01-02"),
		"The current time is: " + time.Now().Format("15:04"),
		"Translate all events to:" + a.Language,
	}

	return append(prompt, promptPreamble...)
}

func PromptCustom(assistant *AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Provide an answer to your employers' question.",
			"Take the following information into account to answer the question:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptWeek(assistant *AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Only include the coming week's events.",
			"Compile a schedule and a summarized overview of todo's, and reminders.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptToday(assistant *AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Start your response with a suitable greeting and comment about today's weather forecast if you have this information. Only include today's and tomorrow's events. Add pointers regarding the weather if relevant. Be verbose.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptFull(assistant *AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(assistant.PromptPreamble(),
		[]string{
			"Add a quick summary of the past week's important events. Add a quick summary of future important events - one line per day. Add weather information for events with outside events.",
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}
