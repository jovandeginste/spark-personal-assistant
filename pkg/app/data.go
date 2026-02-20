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
	ChatHistory      map[string][]ChatHistory `json:",omitempty"`
	EmployerQuestion []string                 `json:",omitempty"`
	UserData         UserData
	FileURIs         []string `json:",omitempty"`
}

func (aiData *AIData) ResetHistory(roomID string) {
	if aiData.ChatHistory == nil {
		aiData.ChatHistory = make(map[string][]ChatHistory)
	}
	aiData.ChatHistory[roomID] = []ChatHistory{}
}

// CleanHistory: keep last 10 elements in aiData.ChatHistory[roomID]:
func (aiData *AIData) CleanHistory(roomID string) {
	if aiData.ChatHistory == nil {
		return
	}

	history, ok := aiData.ChatHistory[roomID]
	if !ok {
		return
	}

	h := []ChatHistory{}
	f := time.Now().Add(-1 * time.Hour)

	for i, e := range history {
		if i < len(history)-100 {
			continue
		}

		if e.time.Before(f) {
			continue
		}

		h = append(h, e)
	}

	aiData.ChatHistory[roomID] = h
}

func (aiData *AIData) AddChatHistory(roomID string, role string, input string) {
	if aiData.ChatHistory == nil {
		aiData.ChatHistory = make(map[string][]ChatHistory)
	}
	aiData.ChatHistory[roomID] = append(
		aiData.ChatHistory[roomID],
		ChatHistory{time: time.Now(), Role: role, Content: input},
	)
}

func (a *App) BuildData() (*AIData, error) {
	aiData := &AIData{
		Context:     a.Config.Context,
		UserData:    a.Config.UserData,
		ChatHistory: make(map[string][]ChatHistory),
	}

	return aiData, nil
}
