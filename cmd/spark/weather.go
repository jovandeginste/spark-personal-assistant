package main

import (
	"github.com/jovandeginste/spark-personal-assistant/pkg/helpers/weather"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/spf13/cobra"
)

func (c *cli) weatherCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "weather2entry source location",
		Short:   "Convert open-meteo JSON to Spark entries",
		Example: "spark weather2entry weather-brussels Brussels",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			geocoder.SetClient(c.app.Logger(), "Spark")

			src, err := c.app.FindSourceByName(args[0])
			if err != nil {
				return err
			}

			entries, err := weather.GetWeatherData(args[1])
			if err != nil {
				return err
			}

			c.app.FetchExistingEntries(src.ID, entries)

			return c.app.ReplaceSourceEntries(src, entries)
		},
	}

	return cmd
}
