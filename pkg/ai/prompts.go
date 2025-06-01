package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

type Prompt func(assistant AssistantConfig, data any) ([]string, error)

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
	"Translate all entries to English.",
	"The following entries consist a list of items.",
	"Entries without a timestamp are for the whole day.",
	"The names in the user data are your employers' names",
}

func (a AssistantConfig) PromptPreamble() []string {
	prompt := []string{
		"Your name is: " + a.Name,
		"Use the following style: " + a.Style,
		"Today is: " + time.Now().Format("Monday, 2006-01-02"),
	}

	return append(prompt, promptPreamble...)
}

func PromptCheckTask(assistant AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	c := []string{
		"Assert whether the employer's question is about creating, updating or deleting one or more tasks.",
		"If no actions are required, answer with a single zero and nothing else.",
		"If actions are required, provide a human readable list of those actions.",
		string(j),
	}

	return c, nil
}

func PromptTask(assistant AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := []string{
		"Execute the employer's question.",
		"Respond with only a JSON array with the following fields for each task:",
		"- action: either add or delete",
		"- date: the date of the task, either YYYY-MM-DD or YYYY-MM-DD HH:MM",
		"- summary: the summary of the task to perform",
		"- description: a more verbose description of the task (omit this field if not needed)",
		"- name: the person or persons for which this task is created, as comma-separated list",
		"Take the following information into account to answer the question:",
		"The names in the user data are your employers' names",
		"Today is: " + time.Now().Format("Monday, 2006-01-02"),
		string(j),
	}

	return c, nil
}

func PromptCustom(assistant AssistantConfig, data any) ([]string, error) {
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
