package main

import (
	"errors"

	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
)

func (c *cli) rssCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rss2entry source https://example.org/feed.xml",
		Short: "Convert rss feed to Spark entries",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			feedURL := args[1]

			fp := gofeed.NewParser()
			fp.UserAgent = "curl/8.12.1"

			feed, err := fp.ParseURL(feedURL)
			if err != nil {
				return err
			}

			if len(feed.Items) == 0 {
				return errors.New("no events")
			}

			var entries data.Entries

			for _, event := range feed.Items {
				entries = append(entries, data.Entry{
					Date:    data.HumanTime{Time: *event.PublishedParsed},
					Summary: event.Title,
				})
			}

			c.app.FetchExistingEntries(entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	return cmd
}
