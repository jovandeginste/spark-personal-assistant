package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func (c *cli) matrixChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "matrix",
		Short:   "Start Matrix client with Spark",
		Example: "spark matrix",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			aiClient, err := ai.NewClient(c.app.Config.LLM, &c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return err
			}

			group, groupCtx := errgroup.WithContext(ctx)
			started := false

			for instanceName, mCfg := range c.app.Config.Matrix {
				if !mCfg.IsEnabled() {
					continue
				}

				started = true

				mc, err := c.initMatrixClient(instanceName, aiClient, nil)
				if err != nil {
					return err
				}

				c.app.Logger().Info(
					"Now running",
					"instance",
					instanceName,
					"client_id", mc.Client.UserID.String(),
					"name", c.app.Config.Assistant.Name,
					"language", c.app.Config.Assistant.Language,
				)

				if wCfg, ok := mc.App.Config.Webserver[instanceName]; ok && wCfg.IsEnabled() {
					mc.ServeHTTP(groupCtx, instanceName)
				}

				group.Go(func() error {
					defer func() { _ = mc.CryptoHelper.Close() }()

					if err := mc.Client.SyncWithContext(groupCtx); err != nil && !errors.Is(err, context.Canceled) {
						return fmt.Errorf("matrix %s: %w", instanceName, err)
					}

					return nil
				})
			}

			if !started {
				return errors.New("no enabled matrix instance found in configuration")
			}

			return group.Wait()
		},
	}

	cmd.Flags().StringVar(&c.app.ConfigFile, "config", "./spark.yaml", "config file")
	cmd.Flags().StringVar(&c.app.Config.AssistantFileCLI, "persona", "", "persona")

	return cmd
}
