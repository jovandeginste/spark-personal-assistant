package main

import (
	"fmt"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
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
	cmd := &cobra.Command{
		Use:          os.Args[0],
		Short:        "Serve Spark",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return c.app.Initialize()
		},
	}

	cmd.AddCommand(c.mailCmd())
	cmd.AddCommand(c.printCmd())

	cmd.PersistentFlags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")

	return cmd
}

func (c *cli) printCmd() *cobra.Command {
	var (
		daysBack  uint
		daysAhead uint
	)

	cmd := &cobra.Command{
		Use:   "print",
		Short: "print summary to screen",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := c.app.CurrentEntries(daysBack, daysAhead)
			if err != nil {
				return err
			}

			md, err := c.app.GeneratePrompt(entries)
			if err != nil {
				return err
			}

			fmt.Println(md)

			return nil
		},
	}

	cmd.Flags().UintVarP(&daysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&daysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}

func (c *cli) mailCmd() *cobra.Command {
	var (
		daysBack  uint
		daysAhead uint
	)

	cmd := &cobra.Command{
		Use:   "mail address",
		Short: "mail summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addresses := args

			entries, err := c.app.CurrentEntries(daysBack, daysAhead)
			if err != nil {
				return err
			}

			md, err := c.app.GeneratePrompt(entries)
			if err != nil {
				return err
			}

			html, err := app.GenerateHTML([]byte(md))
			if err != nil {
				return err
			}

			return c.app.Config.Mailer.Send(addresses, "Daily update", md, string(html))
		},
	}

	cmd.Flags().UintVarP(&daysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&daysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}
