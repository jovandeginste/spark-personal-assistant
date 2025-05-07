package main

import (
	"context"
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/app"
	"github.com/jovandeginste/spark-personal-assistant/pkg/data"
	"github.com/spf13/cobra"
)

type AIData struct {
	ExtraContext     []string
	EmployerQuestion []string `json:"employer_question,omitempty"`
	UserData         app.UserData
	Entries          data.Entries
}

func (c *cli) printCmd() *cobra.Command {
	var (
		ef           app.EntryFilter
		format       string
		customPrompt []string
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

			aiData, err := c.buildData(ef)
			if err != nil {
				return err
			}

			aiData.EmployerQuestion = customPrompt

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

	cmd.Flags().StringSliceVarP(&customPrompt, "prompt", "p", nil, "extra custom prompt")
	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVarP(&format, "format", "f", "full", "Format to use")
	cmd.Flags().UintVarP(&ef.DaysBack, "days-back", "b", 3, "Number of days in the past to include")
	cmd.Flags().UintVarP(&ef.DaysAhead, "days-ahead", "a", 7, "Number of days in the future to include")

	return cmd
}

func (c *cli) buildData(ef app.EntryFilter) (*AIData, error) {
	entries, err := c.app.CurrentEntries(ef)
	if err != nil {
		return nil, err
	}

	aiData := &AIData{
		ExtraContext: c.app.Config.ExtraContext,
		UserData:     c.app.Config.UserData,
		Entries:      entries,
	}

	return aiData, nil
}
