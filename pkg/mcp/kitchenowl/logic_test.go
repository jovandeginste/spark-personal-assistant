package kitchenowl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetDate(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "Standard Date",
			input:    1704067200000, // 2024-01-01 00:00:00 UTC
			expected: "2024-01-01",
		},
		{
			name:     "Another Date",
			input:    1735689600000, // 2025-01-01 00:00:00 UTC
			expected: "2025-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meal := PlannedMeal{
				CookingDateUnixMili: tt.input,
			}
			meal.SetDate()

			assert.Equal(t, tt.expected, meal.CookingDate)
			assert.Equal(t, int64(0), meal.CookingDateUnixMili)
		})
	}
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedError string
	}{
		{
			name: "Valid Config",
			config: Config{
				APIURL: "http://example.com",
				Token:  "some-token",
			},
			expectedError: "",
		},
		{
			name: "Missing Token",
			config: Config{
				APIURL: "http://example.com",
				Token:  "",
			},
			expectedError: "kitchenowl token is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Module{}
			m.SetConfig(tt.config)

			err := m.Enabled()
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
