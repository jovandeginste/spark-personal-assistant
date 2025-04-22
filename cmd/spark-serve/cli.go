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
	cmd := &cobra.Command{
		Use:   "print",
		Short: "print summary to screen",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			es, err := c.app.CurrentEntries()
			if err != nil {
				return err
			}

			md, err := c.app.GeneratePrompt(es)
			if err != nil {
				return err
			}

			fmt.Println(md)

			return nil
		},
	}

	return cmd
}

func (c *cli) mailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail address",
		Short: "mail summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addresses := args

			es, err := c.app.CurrentEntries()
			if err != nil {
				return err
			}

			md, err := c.app.GeneratePrompt(es)
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

	return cmd
}
