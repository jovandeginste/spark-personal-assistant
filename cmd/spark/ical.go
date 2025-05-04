package main

import (
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/ical"
	"github.com/spf13/cobra"
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

			collection := "calendar"
			if len(args) > 2 {
				collection = args[2]
			}

			entries, err := ical.BuildEntriesFromRemote(args[1], daysBack, daysAhead, collection)
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
