package ical

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/apognu/gocal"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/generic"
	"github.com/yaegashi/wtz.go"
)

func BuildEntriesFromRemote(remote string, daysBack, daysAhead uint, collection string) (data.Entries, error) {
	r, err := generic.GetBody(remote)
	if err != nil {
		return nil, err
	}

	return BuildEntriesFromICal(r, daysBack, daysAhead, collection)
}

func BuildEntriesFromICal(r []byte, daysBack, daysAhead uint, collection string) (data.Entries, error) {
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
		t, err := parseICalRawDate(&event.RawStart, event.Start)
		if err != nil {
			return nil, err
		}

		e.Date = data.HumanTime{Time: t}
	}

	e.Summary = event.Summary

	if event.End != nil {
		t, err := parseICalRawDate(&event.RawEnd, event.End)
		if err != nil {
			return nil, err
		}

		d := data.HumanTime{Time: t}
		e.SetMetadata("End", d.FormatDate())
	}

	if event.Start != nil && event.End != nil {
		dur := event.End.Sub(*event.Start)
		e.SetMetadata("Duration", dur.String())
	}

	e.SetMetadataIfNotEmpty("Location", event.Location)
	e.SetMetadataIfNotEmpty("Attendee", collectAttendees(event.Attendees))
	e.SetMetadataIfNotEmpty("Class", event.Class)
	e.SetMetadataIfNotEmpty("Comment", event.Comment)
	e.SetMetadataIfNotEmpty("Description", event.Description)

	if event.Organizer != nil {
		e.SetMetadataIfNotEmpty("Organizer", event.Organizer.Cn)
	}

	t := event.CustomAttributes["TRANSP"]
	e.SetMetadata("Busy", t == "OPAQUE")

	return e, nil
}

func collectAttendees(attendees []gocal.Attendee) string {
	result := make([]string, 0, len(attendees))

	for _, a := range attendees {
		result = append(result, a.Cn)
	}

	return strings.Join(result, ",")
}

func parseICalRawDate(rs *gocal.RawDate, start *time.Time) (time.Time, error) {
	if v, ok := rs.Params["VALUE"]; ok {
		if v == "DATE" {
			return parseICalDate(rs)
		}
	}

	return parseICalTime(rs, start)
}

func parseICalDate(rs *gocal.RawDate) (time.Time, error) {
	return time.Parse("20060102", rs.Value)
}

func parseICalTime(rs *gocal.RawDate, start *time.Time) (time.Time, error) {
	ts, ok := rs.Params["TZID"]
	if !ok {
		return *start, nil
	}

	l := parseTimezone(ts)

	return time.ParseInLocation("20060102T150405", start.Format("20060102T150405"), l)
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
