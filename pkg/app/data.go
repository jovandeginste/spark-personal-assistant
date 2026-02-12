package app

import (
	"time"
)

type ChatHistory struct {
	time    time.Time
	Role    string
	Content string
}

type AIData struct {
	Context          string
	ChatHistory      []ChatHistory `json:",omitempty"`
	EmployerQuestion []string      `json:",omitempty"`
	UserData         UserData
	FileURIs         []string `json:",omitempty"`
}

func (aiData *AIData) ResetHistory() {
	aiData.ChatHistory = []ChatHistory{}
}

// CleanHistory: keep last 10 elements in aiData.ChatHistory:
func (aiData *AIData) CleanHistory() {
	h := []ChatHistory{}
	f := time.Now().Add(-1 * time.Hour)

	for i, e := range aiData.ChatHistory {
		if i < len(aiData.ChatHistory)-100 {
			continue
		}

		if e.time.Before(f) {
			continue
		}

		h = append(h, e)
	}

	aiData.ChatHistory = h
}

func (aiData *AIData) AddChatHistory(role string, input string) {
	aiData.ChatHistory = append(
		aiData.ChatHistory,
		ChatHistory{time: time.Now(), Role: role, Content: input},
	)
}

func (a *App) BuildData() (*AIData, error) {
	aiData := &AIData{
		Context:  a.Config.Context,
		UserData: a.Config.UserData,
	}

	return aiData, nil
}
