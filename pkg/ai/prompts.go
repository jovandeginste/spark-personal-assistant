package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

type Prompt func(data any) ([]string, error)

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
	"You are a personal assistant named 'Spark'.",
	"Use a polite British style and accent.",
	"Use the metric system and 24 hour clock notation.",
	"Use conversational style.",
	"Use emojis.",
	"Translate all entries to English.",
	"You provide an overview in Markdown for your employers.",
	"The following entries consist a list of items.",
}

func PromptPreamble() []string {
	return promptPreamble
}

func PromptWeek(data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(PromptPreamble(),
		[]string{
			"Only include this week's entries.",
			"Compile a schedule and a summarized overview of todo's, and reminders.",
			"Today is: " + time.Now().Format("2006-01-02"),
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptToday(data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(PromptPreamble(),
		[]string{
			"Start your response with a suitable greeting and comment about today's weather forecast if you have this information. Only include today's and tomorrow's entries. Be verbose.",
			"Today is: " + time.Now().Format("2006-01-02"),
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}

func PromptFull(data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := append(PromptPreamble(),
		[]string{
			"Add a quick summary of the past week's important entries. Be verbose about today's entries. Add a quick summary of future important entries - one line per day. Add weather information for days with outside entries.",
			"Today is: " + time.Now().Format("2006-01-02"),
			"Information:",
			string(j),
		}...,
	)

	return c, nil
}
