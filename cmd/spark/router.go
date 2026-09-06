package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mail"
	"github.com/jovandeginste/spark-personal-assistant/pkg/matrix"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
	"github.com/spf13/cobra"
)

func (c *cli) routerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "router",
		Short: "Start the message router for various interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			aiClient, err := ai.NewClient(c.app.Config.LLM, &c.app.Config.Assistant, c.app.Logger())
			if err != nil {
				return fmt.Errorf("failed to initialize AI client: %w", err)
			}

			r := router.NewRouter(aiClient, c.app)

			// Initialize Matrix clients
			for instanceName, mCfg := range c.app.Config.Matrix {
				if !mCfg.IsEnabled() {
					c.app.Logger().Info("Matrix client is disabled in config", "instance", instanceName)
					continue
				}

				mc := matrix.MatrixConfig{App: c.app, InstanceName: instanceName}
				mc.AIClient = aiClient

				aiData, err := c.app.BuildData()
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
				if err := mc.Greet(); err != nil {
					return err
				}

				if err := mc.Register(r, instanceName, "Matrix Room ("+instanceName+")"); err != nil {
					return err
				}

				go func(client *matrix.MatrixConfig) {
					if err := client.Client.SyncWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
						c.app.Logger().Error("Failed to sync matrix", "error", err)
					}
				}(&mc)
			}

			// Initialize Web servers
			for instanceName, wCfg := range c.app.Config.Webserver {
				if wCfg.IsEnabled() {
					c.app.Logger().Info("Initializing Web server...", "instance", instanceName)
					for range c.app.Config.Matrix {
						mc := matrix.MatrixConfig{App: c.app, InstanceName: instanceName}
						mc.AIClient = aiClient
						aiData, _ := c.app.BuildData()
						mc.AIData = aiData
						mc.ServeHTTP(instanceName)
						break
					}

					if err := r.RegisterAddress(router.Address{
						ID:           instanceName + "@web",
						InstanceName: instanceName,
						System:       "web",
						Description:  "Web Server Interface (" + instanceName + ")",
						SendFunc: func(ctx context.Context, msg router.Message) error {
							c.app.Logger().Warn("Web sending not yet supported", "content", msg.Content)
							return nil
						},
					}); err != nil {
						return err
					}
				} else {
					c.app.Logger().Info("Web server is disabled in config", "instance", instanceName)
				}
			}

			// Initialize Mail clients
			for instanceName, mailCfg := range c.app.Config.Mail {
				if !mailCfg.IsEnabled() {
					c.app.Logger().Info("Mail client is disabled in config", "instance", instanceName)
					continue
				}

				c.app.Logger().Info("Initializing Mail client...", "instance", instanceName, "host", mailCfg.IMAP.Host, "port", mailCfg.IMAP.Port)
				mailAddrID := instanceName + "@mail"

				mc := mail.Config{
					IMAP: mail.IMAPConfig{
						Host: mailCfg.IMAP.Host,
						Port: mailCfg.IMAP.Port,
					},
					SMTP: mail.SMTPConfig{
						Host: mailCfg.SMTP.Host,
						Port: mailCfg.SMTP.Port,
					},
					To:       mailCfg.To,
					Username: mailCfg.Username,
					Password: mailCfg.Password,
					Folder:   mailCfg.Folder,
					UseTLS:   mailCfg.UseTLS,
				}

				if err := mail.Register(r, mc, instanceName, "Mail Account ("+instanceName+")", c.app.Logger()); err != nil {
					return err
				}

				go func(cfg mail.Config, addrID string, instName string) {
					msgChan := make(chan mail.Message, 10)
					go func() {
						for msg := range msgChan {
							c.app.Logger().Info("Received email via mail", "instance", instName, "subject", msg.Subject, "from", msg.From)
							_ = r.SubmitMessage(ctx, addrID, router.Message{
								Metadata: router.Metadata{
									To:      []string{"ai"},
									Subject: msg.Subject,
								},
								OriginalSource: msg.FromAddress,
								Content: fmt.Sprintf("I received this email on behalf of my user. Do not reply to the email directly. "+
									"Your response will be sent to the user via Matrix so they know what you did with it.\n\n"+
									"Email from: %s\nSubject: %s\nDate: %s\n\n%s", msg.From, msg.Subject, msg.Date, msg.Body),
							})
						}
					}()

					c.app.Logger().Info("Starting Mail idle connection...", "instance", instName)
					if err := mail.ConnectAndIdle(cfg, msgChan, c.app.Logger()); err != nil {
						c.app.Logger().Error("Mail connection failed", "instance", instName, "error", err)
					}
				}(mc, mailAddrID, instanceName)
			}

			c.app.Logger().Info("Router started")

			// Wait for interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			<-sigChan
			c.app.Logger().Info("Shutting down router...")

			return nil
		},
	}
}
