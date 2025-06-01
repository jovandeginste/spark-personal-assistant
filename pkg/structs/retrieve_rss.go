package structs

import (
	"errors"

	"github.com/jovandeginste/spark-personal-assistant/pkg/humantime"
)

func BuildEntriesFromFeed(feedURL string) (Entries, error) {
	fp.UserAgent = "curl/8.12.1"

	feed, err := fp.ParseURL(feedURL)
	if err != nil {
		return nil, err
	}

	if len(feed.Items) == 0 {
		return nil, errors.New("no events")
	}

	var entries Entries

	for _, event := range feed.Items {
		entries = append(entries, Entry{
			Date:    humantime.HumanTime{Time: *event.PublishedParsed},
			Summary: event.Title,
		})
	}

	return entries, nil
}
