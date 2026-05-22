package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/imap"
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

			// Initialize Matrix client
			mc := matrix.MatrixConfig{App: c.app}
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

			go func() {
				if err := mc.Client.SyncWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
					c.app.Logger().Error("Failed to sync matrix", "error", err)
				}
			}()

			// Initialize Web server
			if c.app.Config.Webserver.Enabled {
				c.app.Logger().Info("Initializing Web server...")
				mc.ServeHTTP()
			} else {
				c.app.Logger().Info("Web server is disabled in config")
			}

			// Initialize IMAP client
			if c.app.Config.IMAP.Enabled {
				c.app.Logger().Info("Initializing IMAP client...", "host", c.app.Config.IMAP.Host, "port", c.app.Config.IMAP.Port)
				go func() {
					msgChan := make(chan imap.Message, 10)
					go func() {
						for msg := range msgChan {
							c.app.Logger().Info("Received email via IMAP", "subject", msg.Subject, "from", msg.From)
							r.SubmitMessage(router.Message{
								Source: "imap",
								Target: "ai",
								Content: fmt.Sprintf("I received this email on behalf of my user. Do not reply to the email directly. "+
									"Your response will be sent to the user via Matrix so they know what you did with it.\n\n"+
									"Email from: %s\nSubject: %s\nDate: %s\n\n%s", msg.From, msg.Subject, msg.Date, msg.Body),
								Metadata: map[string]any{
									"from":    msg.FromAddress,
									"room_id": string(mc.DefaultRoomID()),
									"subject": "Re: " + msg.Subject,
								},
							})
						}
					}()

					c.app.Logger().Info("Starting IMAP idle connection...")
					if err := imap.ConnectAndIdle(imap.Config{
						Host:     c.app.Config.IMAP.Host,
						Port:     c.app.Config.IMAP.Port,
						Username: c.app.Config.IMAP.Username,
						Password: c.app.Config.IMAP.Password,
						Folder:   c.app.Config.IMAP.Folder,
						UseTLS:   c.app.Config.IMAP.UseTLS,
					}, msgChan, c.app.Logger()); err != nil {
						c.app.Logger().Error("IMAP connection failed", "error", err)
					}
				}()
			} else {
				c.app.Logger().Info("IMAP client is disabled in config")
			}

			r.Start(ctx)
			c.app.Logger().Info("Router started")

			// Consume outgoing messages and route them to Matrix if needed
			go func() {
				for msg := range r.OutgoingMessages() {
					c.app.Logger().Info("Received outgoing message from Router", "source", msg.Source, "target", msg.Target)
					if msg.Target == "matrix" {
						c.app.Logger().Info("Forwarding response to Matrix", "content", msg.Content)
						mc.SendMessage(mc.DefaultRoomID(), fmt.Sprintf("Response to %s:\n%s", msg.Target, msg.Content))
					} else if msg.Target == "imap" {
						to, _ := msg.Metadata["from"].(string)
						subject, _ := msg.Metadata["subject"].(string)
						c.app.Logger().Info("Forwarding IMAP response to Matrix", "to", to, "subject", subject)

						mc.SendMessage(mc.DefaultRoomID(), fmt.Sprintf("Processed email from %s (%s). Result:\n%s", to, subject, msg.Content))
					}
				}
			}()

			// Wait for interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			select {
			case <-sigChan:
				c.app.Logger().Info("Shutting down router...")
				cancel()
			}

			return nil
		},
	}
}
