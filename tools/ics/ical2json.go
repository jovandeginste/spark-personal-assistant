package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bounoable/ical"
	"github.com/bounoable/ical/parse"
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

	in, err := ical.Parse(r)
	if err != nil {
		panic(err)
	}

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

func newEvent(event *parse.Event, collection string) (*data.Entry, error) {
	e := &data.Entry{
		Metadata: map[string]any{
			"Collection": collection,
		},
	}

	e.Date = event.Start
	e.Summary = event.Summary

	if eLocation, ok := event.Property("LOCATION"); ok {
		e.Metadata["Location"] = eLocation.Value
	}

	return e, nil
}
