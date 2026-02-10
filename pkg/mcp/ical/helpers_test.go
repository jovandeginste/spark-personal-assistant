package ical

import (
	"testing"
	"time"

	"github.com/apognu/gocal"
	"github.com/stretchr/testify/assert"
)

func TestCleanDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{45 * time.Minute, "45m"},
		{0, "no duration"},
		{time.Second, "no duration"}, // Rounds to minute
		{30 * time.Second, "1m"},     // Rounds up
	}

	for _, tt := range tests {
		result := cleanDuration(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestParseICalDate(t *testing.T) {
	raw := &gocal.RawDate{Value: "20231225"}
	date, err := parseICalDate(raw)
	assert.NoError(t, err)
	assert.Equal(t, 2023, date.Year())
	assert.Equal(t, time.Month(12), date.Month())
	assert.Equal(t, 25, date.Day())
	assert.Equal(t, 0, date.Hour())

	// Error case
	rawErr := &gocal.RawDate{Value: "invalid"}
	_, err = parseICalDate(rawErr)
	assert.Error(t, err)
}

func TestParseTimezone(t *testing.T) {
	// 1. Valid IANA zone
	loc := parseTimezone("America/New_York")
	assert.NotEqual(t, time.UTC, loc)
	assert.Equal(t, "America/New_York", loc.String())

	// 2. Valid Windows zone (wtz)
	locW := parseTimezone("Romance Standard Time")
	assert.NotEqual(t, time.UTC, locW)
	// Output name might vary depending on system, but shouldn't be UTC

	// 3. Invalid zone (fallback to UTC)
	locInv := parseTimezone("Mars/Crater")
	assert.Equal(t, time.UTC, locInv)
}
