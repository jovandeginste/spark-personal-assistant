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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return c.app.Initialize()
		},
	}

	cmd.AddCommand(c.entriesCmd())
	cmd.AddCommand(c.sourcesCmd())
	cmd.AddCommand(c.mailerCmd())
	cmd.AddCommand(c.printCmd())
	cmd.AddCommand(c.md2htmlCmd())
	cmd.AddCommand(c.weatherCmd())
	cmd.AddCommand(c.icalCmd())
	cmd.AddCommand(c.vcfCmd())

	sparkConfig, ok := os.LookupEnv("SPARK_CONFIG")
	if !ok {
		sparkConfig = "./spark.yaml"
	}

	cmd.PersistentFlags().StringVar(&c.app.ConfigFile, "config", sparkConfig, "config file")

	return cmd
}
