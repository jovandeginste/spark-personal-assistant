package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatHistoryPartitioning(t *testing.T) {
	aiData := &AIData{
		ChatHistory: make(map[string][]ChatHistory),
	}

	room1 := "room1"
	room2 := "room2"

	// Test adding history to different rooms
	aiData.AddChatHistory(room1, "user", "hello room 1")
	aiData.AddChatHistory(room2, "user", "hello room 2")

	assert.Equal(t, 1, len(aiData.ChatHistory[room1]), "Room 1 should have 1 message")
	assert.Equal(t, 1, len(aiData.ChatHistory[room2]), "Room 2 should have 1 message")
	assert.Equal(t, "hello room 1", aiData.ChatHistory[room1][0].Content)
	assert.Equal(t, "hello room 2", aiData.ChatHistory[room2][0].Content)

	// Test partitioning: adding to room 1 should not affect room 2
	aiData.AddChatHistory(room1, "assistant", "hi room 1")
	assert.Equal(t, 2, len(aiData.ChatHistory[room1]))
	assert.Equal(t, 1, len(aiData.ChatHistory[room2]))

	// Test ResetHistory
	aiData.ResetHistory(room1)
	assert.Equal(t, 0, len(aiData.ChatHistory[room1]))
	assert.Equal(t, 1, len(aiData.ChatHistory[room2])) // Room 2 stays intact
}

func TestCleanHistory(t *testing.T) {
	aiData := &AIData{
		ChatHistory: make(map[string][]ChatHistory),
	}
	room := "room1"

	// Add old message
	aiData.ChatHistory[room] = []ChatHistory{
		{
			time:    time.Now().Add(-2 * time.Hour),
			Role:    "user",
			Content: "old message",
		},
		{
			time:    time.Now(),
			Role:    "user",
			Content: "new message",
		},
	}

	aiData.CleanHistory(room)

	assert.Equal(t, 1, len(aiData.ChatHistory[room]))
	assert.Equal(t, "new message", aiData.ChatHistory[room][0].Content)
}
