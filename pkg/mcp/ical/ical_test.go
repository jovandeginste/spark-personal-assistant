package ical

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAndCacheICS(t *testing.T) {
	// 1. Setup mock server
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Google Inc//Google Calendar 70.9054//EN
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260101
DTEND;VALUE=DATE:20260102
DTSTAMP:20240101T000000Z
UID:newyear2026@google.com
SUMMARY:New Year's Day
END:VEVENT
END:VCALENDAR`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, icsContent)
	}))
	defer ts.Close()

	// 2. Call fetchAndCacheICS
	// Ensure tmp dir exists or is cleaned up
	os.MkdirAll("./tmp", 0o755)

	defer os.RemoveAll("./tmp")

	cache, err := caching.NewService("./tmp", time.Hour)
	require.NoError(t, err)

	path, err := fetchAndCacheICS(ts.URL, cache)
	require.NoError(t, err)
	assert.Contains(t, path, "tmp/")
	assert.FileExists(t, path)

	// 3. Verify content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, icsContent, string(content))

	// 4. Test Caching (modify server response, expect same file content if within TTL)
	// For this simple test, we just check that calling it again returns the same path
	path2, err := fetchAndCacheICS(ts.URL, cache)
	require.NoError(t, err)
	assert.Equal(t, path, path2)
}

func TestGetEvents(t *testing.T) {
	// 1. Setup mock server
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//EN
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260101
DTEND;VALUE=DATE:20260102
DTSTAMP:20240101T000000Z
UID:event1@example.com
SUMMARY:New Year's Day
END:VEVENT
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260203
DTEND;VALUE=DATE:20260204
DTSTAMP:20240101T000000Z
UID:event2@example.com
SUMMARY:My Birthday
DESCRIPTION:Party time!
LOCATION:Home
END:VEVENT
END:VCALENDAR`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, icsContent)
	}))
	defer ts.Close()

	// Ensure tmp dir exists
	os.MkdirAll("./tmp", 0o755)

	defer os.RemoveAll("./tmp")

	cache, err := caching.NewService("./tmp", time.Hour)
	require.NoError(t, err)

	// 2. Test fetching event on specific date
	targetDate, _ := time.Parse("2006-01-02", "2026-02-03")
	cal := Calendar{Name: "Test Calendar", URL: ts.URL}
	events, err := getEvents(cal, targetDate, targetDate, cache)
	require.NoError(t, err)

	require.Len(t, events, 1)
	assert.Equal(t, "My Birthday", events[0].Summary)
	assert.Equal(t, "Party time!", events[0].Description)
	assert.Equal(t, "Home", events[0].Location)

	// 3. Test date with no events
	emptyDate, _ := time.Parse("2006-01-02", "2026-06-01")
	eventsEmpty, err := getEvents(cal, emptyDate, emptyDate, cache)
	require.NoError(t, err)
	assert.Empty(t, eventsEmpty)

	// 4. Test range query
	startDate, _ := time.Parse("2006-01-02", "2026-01-01")
	endDate, _ := time.Parse("2006-01-02", "2026-02-03")
	eventsRange, err := getEvents(cal, startDate, endDate, cache)
	require.NoError(t, err)
	// Should find "New Year's Day" (Jan 1) and "My Birthday" (Feb 3)
	require.Len(t, eventsRange, 2)
}

func TestSearchEvents(t *testing.T) {
	// 1. Setup mock server
	icsContent := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Example//EN
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260101
DTEND;VALUE=DATE:20260102
DTSTAMP:20240101T000000Z
UID:event1@example.com
SUMMARY:New Year's Day
END:VEVENT
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260203
DTEND;VALUE=DATE:20260204
DTSTAMP:20240101T000000Z
UID:event2@example.com
SUMMARY:My Birthday
DESCRIPTION:Party time!
LOCATION:Home
END:VEVENT
BEGIN:VEVENT
DTSTART;VALUE=DATE:20260315
DTEND;VALUE=DATE:20260316
DTSTAMP:20240101T000000Z
UID:event3@example.com
SUMMARY:Project Deadline
DESCRIPTION:Important delivery
LOCATION:Office
END:VEVENT
END:VCALENDAR`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, icsContent)
	}))
	defer ts.Close()

	// Ensure tmp dir exists
	os.MkdirAll("./tmp", 0o755)

	defer os.RemoveAll("./tmp")

	cache, err := caching.NewService("./tmp", time.Hour)
	require.NoError(t, err)

	// 2. Test simple query match
	cal := Calendar{Name: "Test Calendar", URL: ts.URL}
	events, err := searchEvents(cal, "birthday", "", "", cache)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "My Birthday", events[0].Summary)

	// 3. Test description match
	events, err = searchEvents(cal, "delivery", "", "", cache)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "Project Deadline", events[0].Summary)

	// 4. Test date range filtering
	// Should only find New Year's Day in Jan 2026
	events, err = searchEvents(cal, "Day", "2026-01-01", "2026-01-31", cache)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "New Year's Day", events[0].Summary)

	// Should NOT find New Year's Day if range starts in Feb
	// Note: "My Birthday" is in Feb and matches "Day" via substring in "BirthDAY"
	// So we need to ensure the query doesn't match the Feb event if we want empty result,
	// OR check that we DO find the birthday if we search for Day in Feb.
	// The original intent was to check that "New Year's Day" is NOT found.
	events, err = searchEvents(cal, "Year", "2026-02-01", "2026-12-31", cache)
	require.NoError(t, err)
	assert.Empty(t, events)

	// "My Birthday" is in Feb (today!) and has description "Party time!" - not "Day"
	// But check if "Day" is in "My Birthday" -> Yes, "day" is in "Birthday"
	// Let's use a more specific query for the negative test
	events, err = searchEvents(cal, "Year", "2026-02-01", "2026-12-31", cache)
	require.NoError(t, err)
	assert.Empty(t, events)

	// 5. Test no matches
	events, err = searchEvents(cal, "NonexistentEvent", "", "", cache)
	require.NoError(t, err)
	assert.Empty(t, events)

	// 6. Test calendar metadata match (Name/Description)
	calWithMeta := Calendar{
		Name:        "Work Calendar",
		Description: "Official office schedule",
		URL:         ts.URL,
	}
	// "Official" appears in calendar description, but not in any event
	// Should return all events within the default time range (all 3 events in the mock)
	events, err = searchEvents(calWithMeta, "Official", "", "", cache)
	require.NoError(t, err)
	require.Len(t, events, 3)

	// "Work" appears in calendar name
	events, err = searchEvents(calWithMeta, "Work", "", "", cache)
	require.NoError(t, err)
	require.Len(t, events, 3)
}
