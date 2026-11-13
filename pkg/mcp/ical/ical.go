package ical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/apognu/gocal"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxConcurrentFetches limits how many calendars are fetched at the same time.
const maxConcurrentFetches = 5

type Calendar struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	URL         string `mapstructure:"url"`
}

type Config struct {
	Calendars []Calendar `mapstructure:"calendars"`
}

type Module struct {
	sparkmcp.BaseModule
	Cache caching.Cache
}

func New(config Config, cache caching.Cache, logger *slog.Logger) *Module {
	m := &Module{Cache: cache}
	m.SetConfig(config)
	m.SetLogger(logger.With("module", "ical"))
	return m
}

type icalParams struct {
	sparkmcp.DateRangeParams
}

type searchParams struct {
	Query []string `json:"query" jsonschema:"The search query string(s)"`
	sparkmcp.DateRangeParams
}

type updateParams struct{}

func (m *Module) Register(server *mcp.Server) error {
	logger := m.Logger()
	logger.Info("Registering MCP package")

	// Tool 1: List events for a specific date range
	listTool := &mcp.Tool{
		Name:        "calendar_events",
		Description: "List calendar events for a given date range from the configured ICS calendars",
	}
	mcp.AddTool(server, listTool, m.handleListEvents)

	// Tool 2: Search events
	searchTool := &mcp.Tool{
		Name:        "calendar_search",
		Description: "Search for calendar events matching a query string, with optional date range filters",
	}
	mcp.AddTool(server, searchTool, m.handleSearchEvents)

	// Tool 3: Update calendar cache
	updateTool := &mcp.Tool{
		Name:        "calendar_update",
		Description: "Force update of the calendar cache for all configured calendars",
	}
	mcp.AddTool(server, updateTool, m.handleUpdateCalendar)

	return nil
}

func (m *Module) Enabled() error {
	config := m.Config().(Config)
	if len(config.Calendars) == 0 {
		return errors.New("no ical calendars configured")
	}
	return nil
}

func (m *Module) handleListEvents(ctx context.Context, request *mcp.CallToolRequest, params icalParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "listEvents")

	logger.Debug("List events", "date", params.Date, "start_date", params.StartDate, "end_date", params.EndDate)

	start, end, err := params.ParseDateRange()
	if err != nil {
		logger.Error("Invalid date format", "error", err)
		return nil, nil, fmt.Errorf("invalid date format: %w", err)
	}

	allEvents, failed := collectEvents(logger, config.Calendars, "Failed to get events", func(cal Calendar) ([]Event, error) {
		return m.getEvents(cal, start, end)
	})

	if len(allEvents) == 0 {
		logger.Info("No events found", "start_date", params.StartDate, "end_date", params.EndDate)
		return m.textResult("No events found for this time range." + failedCalendarsNote(failed)), nil, nil
	}

	return m.eventsJSONResult(allEvents)
}

func (m *Module) handleSearchEvents(ctx context.Context, request *mcp.CallToolRequest, params searchParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "searchEvents")

	startDate, endDate := params.GetDateRange()

	logger.Debug("Search events", "query", params.Query, "start_date", startDate, "end_date", endDate)
	allEvents, failed := collectEvents(logger, config.Calendars, "Search failed", func(cal Calendar) ([]Event, error) {
		return m.searchEvents(cal, params.Query, startDate, endDate)
	})

	if len(allEvents) == 0 {
		logger.Info("No matching events found", "query", params.Query)
		return m.textResult("No matching events found." + failedCalendarsNote(failed)), nil, nil
	}

	return m.eventsJSONResult(allEvents)
}

func (m *Module) handleUpdateCalendar(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "updateCalendar")

	logger.Debug("Update calendar")

	errs := make([]error, len(config.Calendars))

	runConcurrently(len(config.Calendars), func(i int) {
		cal := config.Calendars[i]

		_, errs[i] = m.Cache.ForceUpdateFile(cal.URL, func() (io.ReadCloser, error) {
			return generic.ReadResource(cal.URL)
		})
	})

	successCount := 0
	errorMessages := make([]string, 0, len(config.Calendars))

	for i, err := range errs {
		cal := config.Calendars[i]

		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", cal.Name, err))
			logger.Error("Failed to update calendar", "calendar", cal.Name, "error", err)

			continue
		}

		successCount++

		logger.Info("Updated calendar", "calendar", cal.Name)
	}

	resultText := fmt.Sprintf("Updated %d calendars. Errors: %d", successCount, len(errorMessages))
	if len(errorMessages) > 0 {
		resultText += "\nFailures:\n" + strings.Join(errorMessages, "\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: resultText,
			},
		},
	}, nil, nil
}

func (m *Module) Initialize() error {
	m.StartPrefetch()
	return nil
}

// collectEvents fetches events from all calendars concurrently. Calendars that
// fail are logged and skipped, so one unreachable provider does not block the
// others. The returned events keep the configured calendar order.
func collectEvents(logger *slog.Logger, calendars []Calendar, failureMessage string, fetch func(Calendar) ([]Event, error)) ([]Event, []string) {
	results := make([][]Event, len(calendars))
	errs := make([]error, len(calendars))

	runConcurrently(len(calendars), func(i int) {
		results[i], errs[i] = fetch(calendars[i])
	})

	var allEvents []Event

	for _, events := range results {
		allEvents = append(allEvents, events...)
	}

	failed := make([]string, 0, len(calendars))
	for i, err := range errs {
		if err == nil {
			continue
		}

		failed = append(failed, calendars[i].Name)
		logger.Error(failureMessage, "calendar", calendars[i].Name, "error", err)
	}

	return allEvents, failed
}

func failedCalendarsNote(failed []string) string {
	if len(failed) == 0 {
		return ""
	}

	return fmt.Sprintf(" Could not fetch: %s.", strings.Join(failed, ", "))
}

// runConcurrently calls fn for every index below count, running at most
// maxConcurrentFetches calls at the same time.
func runConcurrently(count int, fn func(int)) {
	sem := make(chan struct{}, maxConcurrentFetches)

	var wg sync.WaitGroup

	for i := range count {
		wg.Add(1)

		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			fn(i)
		}()
	}

	wg.Wait()
}

func (m *Module) StartPrefetch() {
	config := m.Config().(Config)
	logger := m.Logger()

	go func() {
		logger.Info("Prefetching calendars", "count", len(config.Calendars))

		runConcurrently(len(config.Calendars), func(i int) {
			cal := config.Calendars[i]

			logger.Info("Prefetching calendar", "name", cal.Name)

			if _, err := m.fetchAndCacheICS(cal.URL); err != nil {
				logger.Error("Failed to prefetch calendar", "name", cal.Name, "error", err)
				return
			}

			logger.Info("Prefetched calendar", "name", cal.Name)
		})
	}()
}

type Event struct {
	CalendarName string              `json:"calendar_name,omitempty"`
	Summary      string              `json:"summary"`
	Start        humantime.HumanTime `json:"start"`
	End          humantime.HumanTime `json:"end"`
	Duration     string              `json:"duration"`
	Description  string              `json:"description,omitempty"`
	Location     string              `json:"location,omitempty"`
}

func (m *Module) getEvents(cal Calendar, start, end time.Time) ([]Event, error) {
	parser, err := m.parseCalendar(cal, start.Add(-24*time.Hour), end.Add(48*time.Hour))
	if err != nil {
		return nil, err
	}

	// Normalize range for filtering
	rangeStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	rangeEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location()).Add(24 * time.Hour)

	results := make([]Event, 0, len(parser.Events))
	for _, e := range parser.Events {
		eventStart := e.Start
		eventEnd := e.End

		if eventStart == nil || eventEnd == nil {
			continue
		}

		// Check overlap: (StartA <= EndB) and (EndA >= StartB)
		if eventStart.After(rangeEnd) || eventEnd.Before(rangeStart) {
			continue
		}

		newEvent, err := eventFromGocal(cal.Name, e)
		if err != nil {
			continue
		}

		results = append(results, newEvent)
	}

	return results, nil
}

func (m *Module) searchEvents(cal Calendar, query []string, startDateStr, endDateStr string) ([]Event, error) {
	start, end, err := determineSearchRange(startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	parser, err := m.parseCalendar(cal, start.Add(-24*time.Hour), end.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}

	queryLower := make([]string, 0, len(query))
	for _, q := range query {
		queryLower = append(queryLower, strings.ToLower(q))
	}
	// Check if any query matches the calendar name/description
	calendarMatch := false
	for _, q := range queryLower {
		if strings.Contains(strings.ToLower(cal.Name), q) || strings.Contains(strings.ToLower(cal.Description), q) {
			calendarMatch = true
			break
		}
	}

	results := make([]Event, 0, len(parser.Events))

	for _, e := range parser.Events {
		if e.Start == nil || e.End == nil {
			continue
		}

		// Filter by date range (inclusive)
		// e.End > start AND e.Start < end
		if !e.End.After(*start) || !e.Start.Before(*end) {
			continue
		}

		// Filter by query
		if !isMatch(e, calendarMatch, queryLower) {
			continue
		}

		newEvent, err := eventFromGocal(cal.Name, e)
		if err != nil {
			continue
		}

		results = append(results, newEvent)
	}

	return results, nil
}

func (m *Module) textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func (m *Module) eventsJSONResult(events []Event) (*mcp.CallToolResult, any, error) {
	jsonEvents, err := json.Marshal(events)
	if err != nil {
		m.Logger().Error("Failed to marshal events", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal events: %w", err)
	}

	return m.textResult(string(jsonEvents)), nil, nil
}

func (m *Module) parseCalendar(cal Calendar, start, end time.Time) (*gocal.Gocal, error) {
	icsPath, err := m.fetchAndCacheICS(cal.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching ICS: %w", err)
	}

	f, err := os.Open(icsPath)
	if err != nil {
		return nil, fmt.Errorf("opening ICS file: %w", err)
	}
	defer f.Close()

	parser := gocal.NewParser(f)
	parser.Start, parser.End = &start, &end

	if err := parser.Parse(); err != nil {
		return nil, fmt.Errorf("parsing ICS: %w", err)
	}

	return parser, nil
}

func eventFromGocal(calendarName string, event gocal.Event) (Event, error) {
	newEvent := Event{
		CalendarName: calendarName,
		Summary:      event.Summary,
		Description:  event.Description,
		Location:     event.Location,
	}

	if err := newEvent.SetTimes(event); err != nil {
		return Event{}, err
	}

	return newEvent, nil
}

func determineSearchRange(startDateStr, endDateStr string) (*time.Time, *time.Time, error) {
	now := time.Now()
	var start, end *time.Time

	if startDateStr != "" {
		t, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_date: %w", err)
		}
		start = &t
	} else {
		// Default: search from 1 year ago if no start date
		t := now.AddDate(-1, 0, 0)
		start = &t
	}

	if endDateStr != "" {
		t, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end_date: %w", err)
		}
		// Make end date inclusive by setting it to the end of the day (start of next day)
		t = t.Add(24 * time.Hour)
		end = &t
	} else {
		// Default: search up to 2 years ahead if no end date
		t := now.AddDate(2, 0, 0)
		end = &t
	}

	return start, end, nil
}

func isMatch(e gocal.Event, calendarMatch bool, queryLower []string) bool {
	if calendarMatch {
		return true
	}

	for _, q := range queryLower {
		switch {
		case strings.Contains(strings.ToLower(e.Summary), q):
			return true
		case strings.Contains(strings.ToLower(e.Description), q):
			return true
		case strings.Contains(strings.ToLower(e.Location), q):
			return true
		}
	}
	return false
}

func (m *Module) fetchAndCacheICS(url string) (string, error) {
	if file, ok := m.Cache.GetFile(url); ok {
		return file, nil
	}

	return m.Cache.SetFile(url, func() (io.ReadCloser, error) {
		return generic.ReadResource(url)
	})
}

func (e *Event) SetTimes(event gocal.Event) error {
	if err := e.SetStart(event); err != nil {
		return err
	}

	if err := e.SetEnd(event); err != nil {
		return err
	}

	e.SetDuration(event)

	return nil
}

func (e *Event) SetStart(event gocal.Event) error {
	s := event.Start
	if s == nil {
		return nil
	}

	t, err := parseICalRawDate(&event.RawStart, event.Start)
	if err != nil {
		return err
	}

	e.Start = sparkmcp.ToHumanTime(t)
	return nil
}

func (e *Event) SetEnd(event gocal.Event) error {
	if event.End == nil {
		return nil
	}
	t, err := parseICalRawDate(&event.RawEnd, event.End)
	if err != nil {
		return err
	}

	e.End = sparkmcp.ToHumanTime(t)
	return nil
}

func (e *Event) SetDuration(event gocal.Event) {
	if event.Start == nil || event.End == nil {
		return
	}

	dur := event.End.Sub(*event.Start)
	e.Duration = cleanDuration(dur)
}
