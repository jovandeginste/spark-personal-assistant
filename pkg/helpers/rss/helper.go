package rss

import (
	"errors"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/mmcdole/gofeed"
)

var fp = gofeed.NewParser()

func BuildEntriesFromFeed(feedURL string) (data.Entries, error) {
	fp.UserAgent = "curl/8.12.1"

	feed, err := fp.ParseURL(feedURL)
	if err != nil {
		return nil, err
	}

	if len(feed.Items) == 0 {
		return nil, errors.New("no events")
	}

	var entries data.Entries

	for _, event := range feed.Items {
		entries = append(entries, data.Entry{
			Date:    data.HumanTime{Time: *event.PublishedParsed},
			Summary: event.Title,
		})
	}

	return entries, nil
}
