package main

import (
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/markdown"
	"github.com/spf13/cobra"
)

func (c *cli) mailerCmd() *cobra.Command {
	var subject string

	cmd := &cobra.Command{
		Use:     "mailer input.md recipient1 recipient2 ...",
		Short:   "Send mails",
		Example: "spark mailer ./md/summary-full.md me@example.com",
		Args:    cobra.MinimumNArgs(2),
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

	cmd.Flags().StringVar(&subject, "subject", "Daily update", "mail subject")

	return cmd
}
