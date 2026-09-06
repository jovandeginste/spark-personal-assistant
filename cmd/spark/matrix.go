package main

import (
	"context"
	"errors"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/matrix"
	"github.com/spf13/cobra"
)

func (c *cli) matrixChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "matrix",
		Short:   "Start Matrix client with Spark",
		Example: "spark matrix",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			aiClient, err := ai.NewClient(c.app.Config.LLM, &c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return err
			}

			aiData, err := c.app.BuildData()
			if err != nil {
				return err
			}

			for instanceName, mCfg := range c.app.Config.Matrix {
				if !mCfg.IsEnabled() {
					continue
				}

				mc := matrix.MatrixConfig{App: c.app, InstanceName: instanceName}
				mc.AIClient = aiClient
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
					"instance",
					instanceName,
					"client_id",
					mc.Client.UserID.String(),
					"name", c.app.Config.Assistant.Name,
					"language", c.app.Config.Assistant.Language,
				)

				if err := mc.Greet(); err != nil {
					return err
				}

				if wCfg, ok := mc.App.Config.Webserver[instanceName]; ok && wCfg.IsEnabled() {
					mc.ServeHTTP(instanceName)
				}

				if err := mc.Client.SyncWithContext(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
					c.app.Logger().Error("Failed to sync", "instance", instanceName, "error", err)
				}

				return mc.CryptoHelper.Close()
			}

			return errors.New("no enabled matrix instance found in configuration")
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")

	return cmd
}
