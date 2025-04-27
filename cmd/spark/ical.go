package main

import (
	"bytes"
	"errors"
	"log"
	"time"

	"github.com/apognu/gocal"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
	"github.com/yaegashi/wtz.go"
)

func (c *cli) icalCmd() *cobra.Command {
	var (
		daysBack  uint
		daysAhead uint
	)

	cmd := &cobra.Command{
		Use:     "ical2entry source url [collection]",
		Short:   "Convert ical to Spark entries",
		Example: "spark ical2entry my-calendar https://example.com/feed/calendar.ics",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			remote := args[1]

			r, err := getBody(remote)
			if err != nil {
				return err
			}

			collection := "calendar"
			if len(args) > 2 {
				collection = args[2]
			}

			entries, err := c.buildEntriesFromICal(r, daysBack, daysAhead, collection)
			if err != nil {
				return err
			}

			c.app.FetchExistingEntries(entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	cmd.Flags().UintVarP(&daysBack, "days-back", "b", 30, "Number of days in the past to include")
	cmd.Flags().UintVarP(&daysAhead, "days-ahead", "a", 120, "Number of days in the future to include")

	return cmd
}

func (c *cli) buildEntriesFromICal(r []byte, daysBack, daysAhead uint, collection string) (data.Entries, error) {
	in := gocal.NewParser(bytes.NewReader(r))
	start := time.Now().Add(-time.Duration(daysBack) * 24 * time.Hour)
	end := time.Now().Add(time.Duration(daysAhead) * 24 * time.Hour)
	in.Start, in.End = &start, &end

	if err := in.Parse(); err != nil {
		return nil, err
	}

	if len(in.Events) == 0 {
		return nil, errors.New("no events")
	}

	var entries data.Entries

	hashes := map[string]bool{}

	for _, event := range in.Events {
		e, err := newEventFromICal(&event, collection)
		if err != nil {
			log.Printf("Error: %s", err)
		}

		if hashes[e.NewRemoteID()] {
			continue
		}

		hashes[e.NewRemoteID()] = true

		entries = append(entries, *e)
	}

	return entries, nil
}

func newEventFromICal(event *gocal.Event, collection string) (*data.Entry, error) {
	e := &data.Entry{}
	e.SetMetadata("Collection", collection)

	if s := event.Start; s != nil {
		t, err := parseICalRawDate(&event.RawStart)
		if err != nil {
			return nil, err
		}

		e.Date = data.HumanTime{Time: t}
	}

	e.Summary = event.Summary

	if event.Location != "" {
		e.SetMetadata("Location", event.Location)
	}

	if event.Organizer != nil {
		e.SetMetadata("Organizer", event.Organizer.Cn)
	}

	if len(event.Attendees) > 0 {
		e.SetMetadata("Attendee", collectAttendees(event.Attendees))
	}

	return e, nil
}

func collectAttendees(attendees []gocal.Attendee) []string {
	var result []string

	for _, a := range attendees {
		result = append(result, a.Cn)
	}

	return result
}

func parseICalRawDate(rs *gocal.RawDate) (time.Time, error) {
	if v, ok := rs.Params["VALUE"]; ok {
		if v == "DATE" {
			return parseICalDate(rs)
		}
	}

	return parseICalTime(rs)
}

func parseICalDate(rs *gocal.RawDate) (time.Time, error) {
	return time.Parse("20060102", rs.Value)
}

func parseICalTime(rs *gocal.RawDate) (time.Time, error) {
	ts, ok := rs.Params["TZID"]
	if !ok {
		return time.Parse("20060102T150405Z", rs.Value)
	}

	l := parseTimezone(ts)

	return time.ParseInLocation("20060102T150405", rs.Value, l)
}

func parseTimezone(tz string) *time.Location {
	if l, err := wtz.LoadLocation(tz); err == nil {
		return l
	}

	if l, err := time.LoadLocation(tz); err == nil {
		return l
	}

	return nil
}
