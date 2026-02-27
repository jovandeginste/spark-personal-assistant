package ai

import (
	"encoding/json"
	"time"
)

type Prompt func(a *AssistantConfig, data any) ([]string, error)

func PromptCustom(a *AssistantConfig, data any) ([]string, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c := []string{
		"Your name is: " + a.Name,
		"Use the following style: " + a.Style,
		"Today is: " + time.Now().Format("Monday, 2006-01-02"),
		"The current time is: " + time.Now().Format("15:04"),
		"The user's preferred language is: " + a.Language,
		"Your entire response should be formatted in Markdown",
		"The names in the user data are your employers' names",
		"Use your tools to collect all relevant information.",
		"Provide an answer to your employers' question.",
		"Take the following information into account to answer the question:",
		string(j),
	}

	return c, nil
}
