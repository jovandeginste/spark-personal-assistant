package main

import (
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/rss"
	"github.com/spf13/cobra"
)

func (c *cli) rssCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rss2entry source https://example.org/feed.xml",
		Short:   "Convert rss feed to Spark entries",
		Example: "spark rss2entry my-feed https://example.org/feed.xml",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			entries, err := rss.BuildEntriesFromFeed(args[1])
			if err != nil {
				return err
			}

			c.app.FetchExistingEntries(entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	return cmd
}
