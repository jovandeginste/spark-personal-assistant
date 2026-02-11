package googlecontacts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/people/v1"
)

func TestDateLogic(t *testing.T) {
	// Create a test person
	person := &people.Person{
		Birthdays: []*people.Birthday{
			{
				Date: &people.Date{
					Month: 5,
					Day:   15,
				},
			},
		},
		Events: []*people.Event{
			{
				Type: "anniversary",
				Date: &people.Date{
					Month: 12,
					Day:   25,
				},
			},
		},
	}

	tests := []struct {
		name       string
		params     dateParams
		start, end string
		want       bool
	}{
		{
			name:  "Exact match birthday",
			start: "05-15",
			end:   "05-15",
			want:  true,
		},
		{
			name:  "Exact match anniversary",
			start: "12-25",
			end:   "12-25",
			want:  true,
		},
		{
			name:  "No match",
			start: "01-01",
			end:   "01-01",
			want:  false,
		},
		{
			name:  "Range match birthday",
			start: "05-01",
			end:   "05-31",
			want:  true,
		},
		{
			name:  "Range match anniversary",
			start: "12-01",
			end:   "12-31",
			want:  true,
		},
		{
			name:  "Range no match",
			start: "06-01",
			end:   "06-30",
			want:  false,
		},
		{
			name:  "Wrap around range match (Dec to Jan)",
			start: "12-20",
			end:   "01-10",
			want:  true,
		},
		{
			name:  "Wrap around range no match",
			start: "12-26",
			end:   "01-10",
			want:  false, // Person is 12-25, range starts 12-26
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesDate(person, tt.start, tt.end)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocationLogic(t *testing.T) {
	person := &people.Person{
		Addresses: []*people.Address{
			{
				FormattedValue: "123 Main St, New York, NY, USA",
				City:           "New York",
				Country:        "USA",
			},
		},
		Locations: []*people.Location{
			{
				Value: "Office",
			},
		},
	}

	tests := []struct {
		name  string
		query []string
		want  bool
	}{
		{
			name:  "Match City",
			query: []string{"New York"},
			want:  true,
		},
		{
			name:  "Match Country",
			query: []string{"usa"},
			want:  true,
		},
		{
			name:  "Match Street",
			query: []string{"Main St"},
			want:  true,
		},
		{
			name:  "No Match",
			query: []string{"London"},
			want:  false,
		},
		{
			name:  "Match Location",
			query: []string{"Office"},
			want:  true,
		},
		{
			name:  "Partial Match Case Insensitive",
			query: []string{"new yor"},
			want:  true,
		},
		{
			name:  "Match One of Multiple",
			query: []string{"London", "New York"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesLocation(person, tt.query)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckDate(t *testing.T) {
	// 5th of May
	d := &people.Date{Month: 5, Day: 5}

	tests := []struct {
		name       string
		start, end string
		want       bool
	}{
		{"Exact match", "05-05", "05-05", true},
		{"Exact mismatch", "05-06", "05-06", false},
		{"In range", "05-01", "05-10", true},
		{"Out range", "06-01", "06-10", false},
		{"Wrap range in", "12-01", "05-06", true},
		{"Wrap range out", "12-01", "05-04", false},
		{"Nil date", "05-05", "05-05", false}, // Special case for manual call
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Nil date" {
				assert.False(t, checkDate(nil, tt.start, tt.end))
			} else {
				assert.Equal(t, tt.want, checkDate(d, tt.start, tt.end))
			}
		})
	}
}

// Note: Testing actual Google API calls requires mocking the Google People API service
// or integration tests with credentials, which is beyond unit testing scope.
// We are focusing on the logic parts (date matching, location matching) here.
