package mcp

import (
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
)

type DateRangeParams struct {
	Date      string `json:"date,omitempty" jsonschema:"A specific date to query (YYYY-MM-DD)"`
	StartDate string `json:"start_date,omitempty" jsonschema:"The start date for the range (YYYY-MM-DD)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"The end date for the range (YYYY-MM-DD)"`
}

// MonthDayRangeParams is used by tools that support recurring annual events.
// It accepts both YYYY-MM-DD and MM-DD.
type MonthDayRangeParams struct {
	Date      string `json:"date,omitempty" jsonschema:"A specific date to query (YYYY-MM-DD, or MM-DD for recurring annual events)"`
	StartDate string `json:"start_date,omitempty" jsonschema:"The start date for the range (YYYY-MM-DD, or MM-DD for recurring annual events)"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"The end date for the range (YYYY-MM-DD, or MM-DD for recurring annual events)"`
}

// GetDateRange returns the start and end dates based on the params.
// If Date is provided, it is used for both start and end.
// If StartDate is provided, it is used for start.
// If EndDate is provided, it is used for end.
// If both Date and StartDate/EndDate are provided, Date takes precedence.
func (p DateRangeParams) GetDateRange() (string, string) {
	return resolveDateRange(p.Date, p.StartDate, p.EndDate)
}

// GetDateRange returns the start and end dates based on the params.
func (p MonthDayRangeParams) GetDateRange() (string, string) {
	return resolveDateRange(p.Date, p.StartDate, p.EndDate)
}

func resolveDateRange(date, startDate, endDate string) (string, string) {
	if date != "" {
		return date, date
	}

	return startDate, endDate
}

// ParseDateRange parses the date range strings into time.Time objects.
// It supports "2006-01-02" format.
// If StartDate is missing, it defaults to now.
// If EndDate is missing, it defaults to StartDate.
func (p DateRangeParams) ParseDateRange() (time.Time, time.Time, error) {
	startStr, endStr := p.GetDateRange()
	var start, end time.Time
	var err error

	if startStr == "" {
		start = time.Now()
	} else {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if endStr == "" {
		end = start
	} else {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return start, end, nil
}

func ToHumanTime(t time.Time) humantime.HumanTime {
	return humantime.HumanTime{Time: t}
}
