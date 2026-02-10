package vcf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDateRange(t *testing.T) {
	tests := []struct {
		name     string
		bd       Date
		start    Date
		end      Date
		expected bool
	}{
		{
			name:     "Within range (same month)",
			bd:       Date{Month: 1, Day: 15},
			start:    Date{Month: 1, Day: 1},
			end:      Date{Month: 1, Day: 31},
			expected: true,
		},
		{
			name:     "Outside range (same month)",
			bd:       Date{Month: 1, Day: 15},
			start:    Date{Month: 1, Day: 1},
			end:      Date{Month: 1, Day: 10},
			expected: false,
		},
		{
			name:     "Within range (cross month)",
			bd:       Date{Month: 2, Day: 1},
			start:    Date{Month: 1, Day: 15},
			end:      Date{Month: 2, Day: 15},
			expected: true,
		},
		{
			name:     "Within range (cross year)",
			bd:       Date{Month: 1, Day: 5},
			start:    Date{Month: 12, Day: 25},
			end:      Date{Month: 1, Day: 10},
			expected: true,
		},
		{
			name:     "Outside range (cross year)",
			bd:       Date{Month: 2, Day: 1},
			start:    Date{Month: 12, Day: 25},
			end:      Date{Month: 1, Day: 10},
			expected: false,
		},
		{
			name:     "Exact match start",
			bd:       Date{Month: 1, Day: 1},
			start:    Date{Month: 1, Day: 1},
			end:      Date{Month: 1, Day: 10},
			expected: true,
		},
		{
			name:     "Exact match end",
			bd:       Date{Month: 1, Day: 10},
			start:    Date{Month: 1, Day: 1},
			end:      Date{Month: 1, Day: 10},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkDateRange(&tt.bd, tt.start, tt.end)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    string
		expected Date
		wantErr  bool
	}{
		{"01-01", Date{Month: 1, Day: 1}, false},
		{"12-31", Date{Month: 12, Day: 31}, false},
		{"invalid", Date{}, true},
		{"1-1", Date{Month: 1, Day: 1}, false},
		{"01-01-2022", Date{}, true}, // Should fail, expects MM-DD
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDate(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
