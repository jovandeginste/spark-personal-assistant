package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	ical "github.com/arran4/golang-ical"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
)

func main() {
	if len(os.Args) < 2 {
		log.Println("Usage:", os.Args[0], "<file.ics> [collection]")
		return
	}

	file := os.Args[1]

	collection := "calendar"
	if len(os.Args) > 2 {
		collection = os.Args[2]
	}

	r, err := os.Open(file)
	if err != nil {
		panic(err)
	}

	defer r.Close()

	in, err := ical.ParseCalendar(r)
	if err != nil {
		panic(err)
	}

	var results []*data.Entry
	hashes := map[string]bool{}

	for _, event := range in.Events() {
		e, err := newEvent(event, collection)
		if err != nil {
			log.Printf("Error: %s", err)
			continue
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

func newEvent(event *ical.VEvent, collection string) (*data.Entry, error) {
	eDate, err := event.GetStartAt()
	if err != nil {
		return nil, err
	}

	e := &data.Entry{
		Date: eDate,
		Metadata: map[string]any{
			"Collection": collection,
		},
	}

	if eSummary := event.GetProperty("SUMMARY"); eSummary != nil {
		e.Summary = eSummary.Value
	} else {
		return nil, errors.New("event has no summary")
	}

	if eLocation := event.GetProperty("LOCATION"); eLocation != nil {
		e.Metadata["Location"] = eLocation.Value
	}

	return e, nil
}
