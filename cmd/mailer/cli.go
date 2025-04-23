package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
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
	var subject string

	cmd := &cobra.Command{
		Use:          os.Args[0],
		Short:        "Send mails",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			addresses := args[1:]

			md, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			html, err := markdown.GenerateHTML(md)
			if err != nil {
				return err
			}

			return c.app.Config.Mailer.Send(addresses, subject, string(md), string(html))
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&subject, "subject", "Daily update", "mail subject")

	return cmd
}
