package main

import (
	"context"
	"errors"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/matrix"
	"github.com/spf13/cobra"
)

func (c *cli) matrixChatCmd() *cobra.Command {
	mc := matrix.MatrixConfig{App: c.app}

	cmd := &cobra.Command{
		Use:     "matrix",
		Short:   "Start Matrix client with Spark",
		Example: "spark matrix",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := mc.App.FindSourceByName(mc.Source)
			if err != nil {
				return err
			}

			mc.SourceID = src.ID

			aiClient, err := ai.NewClient(c.app.Config.LLM, c.app.Config.Assistant)
			if err != nil {
				return err
			}

			mc.AIClient = aiClient
			aiData, err := c.app.BuildData(&mc.EF)
			if err != nil {
				return err
			}

			mc.AIData = aiData

			if err := mc.InitClient(); err != nil {
				return err
			}

			mc.ConfigureSyncer()

			if err := mc.ConfigureCryptoHelper(); err != nil {
				return err
			}

			mc.InitChat()
			c.app.Logger().Info(
				"Now running",
				"client_id",
				mc.Client.UserID.String(),
				"name", c.app.Config.Assistant.Name,
			)

			if err := mc.Greet(); err != nil {
				return err
			}

			if err := mc.Client.SyncWithContext(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
				c.app.Logger().Error("Failed to sync", "error", err)
			}

			return mc.CryptoHelper.Close()
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")
	cmd.Flags().StringVar(&mc.Database, "database", "mautrix-example.db", "SQLite database path")
	cmd.Flags().StringVar(&mc.Source, "source", "custom", "Name of the source to use")
	cmd.Flags().UintVarP(&mc.EF.DaysBack, "days-back", "b", 30, "Number of days in the past to include")
	cmd.Flags().UintVarP(&mc.EF.DaysAhead, "days-ahead", "a", 90, "Number of days in the future to include")

	return cmd
}
