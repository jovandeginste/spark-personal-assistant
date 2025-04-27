package main

import (
	"context"
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

func (c *cli) printCmd() *cobra.Command {
	var (
		daysBack  uint
		daysAhead uint
		format    string
	)

	cmd := &cobra.Command{
		Use:     "print",
		Short:   "Print Spark summary",
		Example: "spark print",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.app.Initialize(); err != nil {
				return err
			}

			aiData := c.buildData(daysBack, daysAhead)

			p, err := ai.PromptFor(format)
			if err != nil {
				return err
			}

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			c.app.Logger().Info(
				"Generating summary for entries...",
				"type", c.app.Config.LLM.Type,
				"model", c.app.Config.LLM.Model,
			)

			md, err := aiClient.GeneratePrompt(context.Background(), p, aiData)
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

func (c *cli) buildData(daysBack, daysAhead uint) any {
	entries, err := c.app.CurrentEntries(daysBack, daysAhead)
	if err != nil {
		return err
	}

	aiData := struct {
		ExtraContext []string
		EmployerData app.EmployerData
		Entries      data.Entries
	}{
		ExtraContext: c.app.Config.ExtraContext,
		EmployerData: c.app.Config.EmployerData,
		Entries:      entries,
	}

	return aiData
}
