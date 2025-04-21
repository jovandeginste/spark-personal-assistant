package main

import (
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
		Short:        "Manage Spark",
		SilenceUsage: true,
	}

	cmd.AddCommand(c.entriesCmd())
	cmd.AddCommand(c.sourcesCmd())

	return cmd
}
