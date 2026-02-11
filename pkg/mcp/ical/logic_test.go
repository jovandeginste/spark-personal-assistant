package ical

import (
	"strings"
	"testing"
	"time"

	"github.com/apognu/gocal"
	"github.com/stretchr/testify/assert"
)

func TestDetermineSearchRange(t *testing.T) {
	tests := []struct {
		name      string
		start     string
		end       string
		expectErr bool
	}{
		{
			name:      "Valid Range",
			start:     "2023-01-01",
			end:       "2023-12-31",
			expectErr: false,
		},
		{
			name:      "Invalid Start Date",
			start:     "invalid",
			end:       "2023-12-31",
			expectErr: true,
		},
		{
			name:      "Invalid End Date",
			start:     "2023-01-01",
			end:       "invalid",
			expectErr: true,
		},
		{
			name:      "Empty Dates (Defaults)",
			start:     "",
			end:       "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, err := determineSearchRange(tt.start, tt.end)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, s)
				assert.NotNil(t, e)
			}
		})
	}
}

func TestIsMatch(t *testing.T) {
	evt := gocal.Event{
		Summary:     "Meeting with Bob",
		Description: "Discuss project Alpha",
		Location:    "Room 101",
	}

	tests := []struct {
		name          string
		query         []string
		calendarMatch bool
		expected      bool
	}{
		{"Match Summary", []string{"meeting"}, false, true},
		{"Match Description", []string{"alpha"}, false, true},
		{"Match Location", []string{"101"}, false, true},
		{"No Match", []string{"xyz"}, false, false},
		{"Calendar Match", []string{"xyz"}, true, true},
		{"Case Insensitive", []string{"BOB"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pre-convert queries to lower case as the function expects
			queriesLower := make([]string, 0, len(tt.query))
			for _, q := range tt.query {
				queriesLower = append(queriesLower, strings.ToLower(q))
			}
			result := isMatch(evt, tt.calendarMatch, queriesLower)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetTimes(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	gocalEvt := gocal.Event{
		Start: &now,
		End:   &later,
	}

	evt := Event{}
	err := evt.SetTimes(gocalEvt)
	assert.NoError(t, err)
	assert.Equal(t, now, evt.Start)
	assert.Equal(t, later, evt.End)
	// Duration logic might need specific check depending on cleanDuration implementation
	// But basic invocation should pass
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
				Calendars: []Calendar{{Name: "Test"}},
			},
			expectedError: "",
		},
		{
			name: "No Calendars",
			config: Config{
				Calendars: []Calendar{},
			},
			expectedError: "no ical calendars configured",
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
