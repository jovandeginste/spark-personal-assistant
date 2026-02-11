package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetDateRange(t *testing.T) {
	tests := []struct {
		name      string
		params    DateRangeParams
		wantStart string
		wantEnd   string
	}{
		{
			name:      "Empty params",
			params:    DateRangeParams{},
			wantStart: "",
			wantEnd:   "",
		},
		{
			name: "Date provided",
			params: DateRangeParams{
				Date: "2023-01-01",
			},
			wantStart: "2023-01-01",
			wantEnd:   "2023-01-01",
		},
		{
			name: "StartDate provided",
			params: DateRangeParams{
				StartDate: "2023-01-01",
			},
			wantStart: "2023-01-01",
			wantEnd:   "",
		},
		{
			name: "EndDate provided",
			params: DateRangeParams{
				EndDate: "2023-01-01",
			},
			wantStart: "",
			wantEnd:   "2023-01-01",
		},
		{
			name: "StartDate and EndDate provided",
			params: DateRangeParams{
				StartDate: "2023-01-01",
				EndDate:   "2023-01-31",
			},
			wantStart: "2023-01-01",
			wantEnd:   "2023-01-31",
		},
		{
			name: "Date takes precedence over StartDate/EndDate",
			params: DateRangeParams{
				Date:      "2023-01-15",
				StartDate: "2023-01-01",
				EndDate:   "2023-01-31",
			},
			wantStart: "2023-01-15",
			wantEnd:   "2023-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := tt.params.GetDateRange()
			assert.Equal(t, tt.wantStart, gotStart)
			assert.Equal(t, tt.wantEnd, gotEnd)
		})
	}
}

func TestParseDateRange(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		params    DateRangeParams
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "Empty params (defaults to now)",
			params:    DateRangeParams{},
			wantStart: now, // Approximate check needed
			wantEnd:   now, // Approximate check needed
			wantErr:   false,
		},
		{
			name: "Valid Date",
			params: DateRangeParams{
				Date: "2023-01-01",
			},
			wantStart: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name: "Valid StartDate only (End defaults to Start)",
			params: DateRangeParams{
				StartDate: "2023-01-01",
			},
			wantStart: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name: "Valid StartDate and EndDate",
			params: DateRangeParams{
				StartDate: "2023-01-01",
				EndDate:   "2023-01-31",
			},
			wantStart: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name: "Invalid Date format",
			params: DateRangeParams{
				Date: "invalid-date",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := tt.params.ParseDateRange()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.name == "Empty params (defaults to now)" {
					// Check if within reasonable range (e.g. 1 second)
					assert.WithinDuration(t, tt.wantStart, gotStart, time.Second)
					assert.WithinDuration(t, tt.wantEnd, gotEnd, time.Second)
				} else {
					assert.Equal(t, tt.wantStart, gotStart)
					assert.Equal(t, tt.wantEnd, gotEnd)
				}
			}
		})
	}
}
