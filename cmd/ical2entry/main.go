package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apognu/gocal"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/yaegashi/wtz.go"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage:", os.Args[0], "<file.ics> [collection]")
		return
	}

	file := os.Args[1]

	start, end := time.Now().Add(-30*24*time.Hour), time.Now().Add(60*24*time.Hour)

	collection := "calendar"
	if len(os.Args) > 2 {
		collection = os.Args[2]
	}

	r, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	defer r.Close()

	in := gocal.NewParser(r)
	in.Start, in.End = &start, &end
	in.Parse()

	var results []*data.Entry
	hashes := map[string]bool{}

	for _, event := range in.Events {
		e, err := newEvent(&event, collection)
		if err != nil {
			log.Printf("Error: %s", err)
		}

		if hashes[e.NewRemoteID()] {
			continue
		}

		hashes[e.NewRemoteID()] = true
		results = append(results, e)
	}

	out, err := json.Marshal(results)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
}

func newEvent(event *gocal.Event, collection string) (*data.Entry, error) {
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
