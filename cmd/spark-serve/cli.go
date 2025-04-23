package main

import (
	"fmt"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

type cli struct {
	app     *app.App
	rootCmd *cobra.Command
}

func NewCLI(a *app.App) *cli {
	c := &cli{app: a}

	c.rootCmd = c.root()

	return c
}

func (c *cli) root() *cobra.Command {
	var (
		daysBack  uint
		daysAhead uint
		format    string
	)

	cmd := &cobra.Command{
		Use:          os.Args[0],
		Short:        "Generate Spark entries",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			entries, err := c.app.CurrentEntries(daysBack, daysAhead)
			if err != nil {
				return err
			}

			data := struct {
				EmployerData app.EmployerData
				Entries      data.Entries
			}{
				EmployerData: c.app.Config.EmployerData,
				Entries:      entries,
			}

			p, err := ai.PromptFor(format)
			if err != nil {
				return err
			}

			md, err := ai.GeneratePrompt(p, data)
			if err != nil {
				return err
			}

			fmt.Println(md)

			return nil
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVarP(&format, "format", "f", "full", "Format to use")
	cmd.Flags().UintVarP(&daysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&daysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}
