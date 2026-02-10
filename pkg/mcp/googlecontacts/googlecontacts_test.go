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
		name   string
		params dateParams
		want   bool
	}{
		{
			name:   "Exact match birthday",
			params: dateParams{Date: "05-15"},
			want:   true,
		},
		{
			name:   "Exact match anniversary",
			params: dateParams{Date: "12-25"},
			want:   true,
		},
		{
			name:   "No match",
			params: dateParams{Date: "01-01"},
			want:   false,
		},
		{
			name:   "Range match birthday",
			params: dateParams{StartDate: "05-01", EndDate: "05-31"},
			want:   true,
		},
		{
			name:   "Range match anniversary",
			params: dateParams{StartDate: "12-01", EndDate: "12-31"},
			want:   true,
		},
		{
			name:   "Range no match",
			params: dateParams{StartDate: "06-01", EndDate: "06-30"},
			want:   false,
		},
		{
			name:   "Wrap around range match (Dec to Jan)",
			params: dateParams{StartDate: "12-20", EndDate: "01-10"},
			want:   true,
		},
		{
			name:   "Wrap around range no match",
			params: dateParams{StartDate: "12-26", EndDate: "01-10"},
			want:   false, // Person is 12-25, range starts 12-26
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesDate(person, tt.params)
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
		query string
		want  bool
	}{
		{
			name:  "Match City",
			query: "New York",
			want:  true,
		},
		{
			name:  "Match Country",
			query: "usa",
			want:  true,
		},
		{
			name:  "Match Street",
			query: "Main St",
			want:  true,
		},
		{
			name:  "No Match",
			query: "London",
			want:  false,
		},
		{
			name:  "Match Location",
			query: "Office",
			want:  true,
		},
		{
			name:  "Partial Match Case Insensitive",
			query: "new yor",
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
		name   string
		params dateParams
		want   bool
	}{
		{"Exact match", dateParams{Date: "05-05"}, true},
		{"Exact mismatch", dateParams{Date: "05-06"}, false},
		{"In range", dateParams{StartDate: "05-01", EndDate: "05-10"}, true},
		{"Out range", dateParams{StartDate: "06-01", EndDate: "06-10"}, false},
		{"Wrap range in", dateParams{StartDate: "12-01", EndDate: "05-06"}, true},
		{"Wrap range out", dateParams{StartDate: "12-01", EndDate: "05-04"}, false},
		{"Nil date", dateParams{Date: "05-05"}, false}, // Special case for manual call
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Nil date" {
				assert.False(t, checkDate(nil, tt.params))
			} else {
				assert.Equal(t, tt.want, checkDate(d, tt.params))
			}
		})
	}
}

// Note: Testing actual Google API calls requires mocking the Google People API service
// or integration tests with credentials, which is beyond unit testing scope.
// We are focusing on the logic parts (date matching, location matching) here.
