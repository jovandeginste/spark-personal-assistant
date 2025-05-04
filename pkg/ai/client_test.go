package ai

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	assistantConfig := AssistantConfig{
		Name:  "TestAssistant",
		Style: "TestStyle",
	}

	tests := []struct {
		name               string
		aiConfig           *AIConfig
		assistantConfig    AssistantConfig
		expectError        bool
		expectedClientType Client // Use reflect.Type to check the concrete type
	}{
		{
			name: "Create Gemini client",
			aiConfig: &AIConfig{
				Type:   "gemini",
				APIKey: "gemini-key",
				Model:  "gemini-pro",
			},
			assistantConfig:    assistantConfig,
			expectError:        false,
			expectedClientType: geminiClient{}, // Expected concrete type
		},
		{
			name: "Create OpenAI client",
			aiConfig: &AIConfig{
				Type:   "openai",
				APIKey: "openai-key",
				Model:  "gpt-4",
			},
			assistantConfig:    assistantConfig,
			expectError:        false,
			expectedClientType: openaiClient{}, // Expected concrete type
		},
		{
			name: "Create Ollama client",
			aiConfig: &AIConfig{
				Type:  "ollama",
				Model: "llama3", // Ollama typically doesn't use API keys directly in this config
			},
			assistantConfig:    assistantConfig,
			expectError:        false,
			expectedClientType: ollamaClient{}, // Expected concrete type
		},
		{
			name: "Unknown AI type",
			aiConfig: &AIConfig{
				Type: "unknown",
			},
			assistantConfig:    assistantConfig,
			expectError:        true,
			expectedClientType: nil, // No client expected on error
		},
		{
			name:               "Nil AIConfig", // Should panic or error if AIConfig is nil
			aiConfig:           nil,
			assistantConfig:    assistantConfig,
			expectError:        true, // Dereferencing nil AIConfig will cause panic, which test can catch as error
			expectedClientType: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use defer recover to catch potential panics from nil pointer dereference
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectError {
						t.Fatalf("NewClient panicked unexpectedly: %v", r)
					}
					// If expecting an error (due to nil config), a panic is acceptable behavior for this case
					assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer dereference", "Panic was not due to nil pointer dereference")
				} else if tt.expectError && tt.aiConfig != nil && tt.aiConfig.Type != "unknown" {
					t.Fatalf("NewClient did not panic as expected for nil AIConfig, but should have erred")
				}
			}()

			client, err := NewClient(tt.aiConfig, tt.assistantConfig)

			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Nil(t, client, "Expected nil client on error")
				if tt.aiConfig != nil && tt.aiConfig.Type != "unknown" {
					// Specific error check for unknown type
					assert.Contains(t, err.Error(), "unknown type", "Error message did not indicate unknown type")
				}
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.NotNil(t, client, "Expected non-nil client on success")

				// Assert the concrete type of the returned interface
				assert.IsType(t, tt.expectedClientType, client, "Returned client is not of the expected concrete type")

				// Optionally, test if basic methods work on the created client (APIKey, Model)
				// This implicitly tests if the correct struct was initialized
				if tt.aiConfig != nil {
					assert.Equal(t, tt.aiConfig.APIKey, client.APIKey(), "APIKey mismatch")
					assert.Equal(t, tt.aiConfig.Model, client.Model(), "Model mismatch")
				}
			}
		})
	}
}
