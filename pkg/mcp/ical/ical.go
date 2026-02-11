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
	"time"

	"github.com/apognu/gocal"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	Query string `json:"query" jsonschema:"The search query string"`
	sparkmcp.DateRangeParams
}

type updateParams struct {
	Force bool `json:"force,omitempty" jsonschema:"Optional: Force update (default true)"`
}

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

	var allEvents []Event
	for _, cal := range config.Calendars {
		events, err := m.getEvents(cal, start, end)
		if err != nil {
			logger.Error("Failed to get events", "calendar", cal.Name, "error", err)
			return nil, nil, fmt.Errorf("failed to get events from %s: %w", cal.Name, err)
		}

		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		logger.Info("No events found", "start_date", params.StartDate, "end_date", params.EndDate)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No events found for this time range.",
				},
			},
		}, nil, nil
	}

	jsonEvents, err := json.Marshal(allEvents)
	if err != nil {
		logger.Error("Failed to marshal events", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal events: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonEvents),
			},
		},
	}, nil, nil
}

func (m *Module) handleSearchEvents(ctx context.Context, request *mcp.CallToolRequest, params searchParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "searchEvents")

	startDate, endDate := params.GetDateRange()

	logger.Debug("Search events", "query", params.Query, "start_date", startDate, "end_date", endDate)
	var allEvents []Event
	for _, cal := range config.Calendars {
		events, err := m.searchEvents(cal, params.Query, startDate, endDate)
		if err != nil {
			logger.Error("Search failed", "calendar", cal.Name, "error", err)
			return nil, nil, fmt.Errorf("search failed on %s: %w", cal.Name, err)
		}

		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		logger.Info("No matching events found", "query", params.Query)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No matching events found.",
				},
			},
		}, nil, nil
	}

	jsonEvents, err := json.Marshal(allEvents)
	if err != nil {
		logger.Error("Failed to marshal search results", "error", err)
		return nil, nil, fmt.Errorf("failed to marshal search results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonEvents),
			},
		},
	}, nil, nil
}

func (m *Module) handleUpdateCalendar(ctx context.Context, request *mcp.CallToolRequest, params updateParams) (*mcp.CallToolResult, any, error) {
	config := m.Config().(Config)
	logger := m.Logger().With("handler", "updateCalendar")

	logger.Debug("Update calendar", "force", params.Force)
	successCount := 0
	errorCount := 0

	var errorMessages []string

	for _, cal := range config.Calendars {
		_, err := m.Cache.ForceUpdateFile(cal.URL, func() (io.ReadCloser, error) {
			return generic.ReadResource(cal.URL)
		})
		if err != nil {
			errorCount++

			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", cal.Name, err))
			logger.Error("Failed to update calendar", "calendar", cal.Name, "error", err)
		} else {
			successCount++

			logger.Info("Updated calendar", "calendar", cal.Name)
		}
	}

	resultText := fmt.Sprintf("Updated %d calendars. Errors: %d", successCount, errorCount)
	if errorCount > 0 {
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

func (m *Module) StartPrefetch() {
	config := m.Config().(Config)
	logger := m.Logger()

	go func() {
		logger.Info("Prefetching calendars", "count", len(config.Calendars))

		for _, cal := range config.Calendars {
			logger.Info("Prefetching calendar", "name", cal.Name)
			// Trigger cache update by calling fetch logic
			if _, err := m.fetchAndCacheICS(cal.URL); err != nil {
				logger.Error("Failed to prefetch calendar", "name", cal.Name, "error", err)
			} else {
				// We can read file size or something to log
				// For now just success log
				logger.Info("Prefetched calendar", "name", cal.Name)
			}
		}
	}()
}

type Event struct {
	CalendarName string    `json:"calendar_name,omitempty"`
	Summary      string    `json:"summary"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	Duration     string    `json:"duration"`
	Description  string    `json:"description,omitempty"`
	Location     string    `json:"location,omitempty"`
}

func (m *Module) getEvents(cal Calendar, start, end time.Time) ([]Event, error) {
	icsPath, err := m.fetchAndCacheICS(cal.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching ICS: %w", err)
	}

	f, err := os.Open(icsPath)
	if err != nil {
		return nil, fmt.Errorf("opening ICS file: %w", err)
	}
	defer f.Close()

	// Parse with buffer around the range
	pStart := start.Add(-24 * time.Hour)
	pEnd := end.Add(48 * time.Hour)

	parser := gocal.NewParser(f)
	parser.Start, parser.End = &pStart, &pEnd

	if err := parser.Parse(); err != nil {
		return nil, fmt.Errorf("parsing ICS: %w", err)
	}

	// Normalize range for filtering
	rangeStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	rangeEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location()).Add(24 * time.Hour)

	var results []Event
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

		newEvent := Event{
			CalendarName: cal.Name,
			Summary:      e.Summary,
			Description:  e.Description,
			Location:     e.Location,
		}
		if err := newEvent.SetTimes(e); err != nil {
			continue
		}

		results = append(results, newEvent)
	}

	return results, nil
}

func (m *Module) searchEvents(cal Calendar, query string, startDateStr, endDateStr string) ([]Event, error) {
	icsPath, err := m.fetchAndCacheICS(cal.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching ICS: %w", err)
	}

	f, err := os.Open(icsPath)
	if err != nil {
		return nil, fmt.Errorf("opening ICS file: %w", err)
	}
	defer f.Close()

	start, end, err := determineSearchRange(startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	// Widen the parser scope to ensure we don't miss boundary events
	pStart := start.Add(-24 * time.Hour)
	pEnd := end.Add(24 * time.Hour)
	parser := gocal.NewParser(f)
	parser.Start, parser.End = &pStart, &pEnd

	if err := parser.Parse(); err != nil {
		return nil, fmt.Errorf("parsing ICS: %w", err)
	}

	queryLower := strings.ToLower(query)
	calendarMatch := strings.Contains(strings.ToLower(cal.Name), queryLower) || strings.Contains(strings.ToLower(cal.Description), queryLower)

	var results []Event

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

		newEvent := Event{
			CalendarName: cal.Name,
			Summary:      e.Summary,
			Description:  e.Description,
			Location:     e.Location,
		}

		newEvent.SetTimes(e)

		results = append(results, newEvent)
	}

	return results, nil
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

func isMatch(e gocal.Event, calendarMatch bool, queryLower string) bool {
	if calendarMatch {
		return true
	}

	switch {
	case strings.Contains(strings.ToLower(e.Summary), queryLower):
		return true
	case strings.Contains(strings.ToLower(e.Description), queryLower):
		return true
	case strings.Contains(strings.ToLower(e.Location), queryLower):
		return true
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

	e.Start = t
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

	e.End = t
	return nil
}

func (e *Event) SetDuration(event gocal.Event) {
	if event.Start == nil || event.End == nil {
		return
	}

	dur := event.End.Sub(*event.Start)
	e.Duration = cleanDuration(dur)
}
