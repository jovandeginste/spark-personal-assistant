package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mail"
	"github.com/jovandeginste/spark-personal-assistant/pkg/matrix"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
	"github.com/jovandeginste/spark-personal-assistant/pkg/web"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"
	"github.com/spf13/cobra"
)

const mailRetryDelay = time.Minute

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

			var wg sync.WaitGroup

			if err := c.startMatrixRouters(ctx, aiClient, r, &wg); err != nil {
				return err
			}

			if err := c.startWebServers(ctx, r, &wg); err != nil {
				return err
			}

			if err := c.startMailRouters(ctx, r, &wg); err != nil {
				return err
			}

			c.app.Logger().Info("Router started")
			c.waitForShutdown()

			cancel()
			wg.Wait()

			return nil
		},
	}
}

func (c *cli) startMatrixRouters(ctx context.Context, aiClient ai.Client, r *router.Router, wg *sync.WaitGroup) error {
	for instanceName, mCfg := range c.app.Config.Matrix {
		if !mCfg.IsEnabled() {
			c.app.Logger().Info("Matrix client is disabled in config", "instance", instanceName)
			continue
		}

		mc, err := c.initMatrixClient(instanceName, aiClient, r)
		if err != nil {
			return err
		}

		wg.Add(1)

		go func(client *matrix.MatrixConfig) {
			defer wg.Done()

			if err := client.Client.SyncWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				c.app.Logger().Error("Failed to sync matrix", "error", err)
			}
		}(mc)
	}

	return nil
}

func (c *cli) startWebServers(ctx context.Context, r *router.Router, wg *sync.WaitGroup) error {
	for instanceName, wCfg := range c.app.Config.Webserver {
		if !wCfg.IsEnabled() {
			c.app.Logger().Info("Web server is disabled in config", "instance", instanceName)
			continue
		}

		c.app.Logger().Info("Initializing Web server...", "instance", instanceName)

		webAddrID := instanceName + "@web"
		if err := r.RegisterAddress(router.Address{
			ID:           webAddrID,
			InstanceName: instanceName,
			System:       "web",
			Description:  "Web Server Interface (" + instanceName + ")",
			SendFunc: func(ctx context.Context, msg router.Message) error {
				c.app.Logger().Info("Web message response", "content", msg.Content)
				return nil
			},
		}); err != nil {
			return err
		}

		e := echo.New()
		e.Use(slogecho.New(c.app.Logger()))
		e.Use(middleware.Recover())
		e.Use(web.TokenAuth(wCfg.Token))
		e.HideBanner = true
		e.HidePort = true

		e.GET("/prompt", func(ec echo.Context) error {
			prompt := ec.QueryParam("prompt")
			if prompt == "" {
				return ec.JSON(http.StatusBadRequest, map[string]string{"error": "prompt query parameter is required"})
			}
			return submitWebPrompt(ec, r, webAddrID, "Web Prompt", prompt)
		})

		e.POST("/prompt", func(ec echo.Context) error {
			var payload web.PromptPayload
			if err := ec.Bind(&payload); err != nil {
				return ec.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON format"})
			}
			if payload.Title == "" {
				return ec.JSON(http.StatusBadRequest, map[string]string{"error": "Title is required"})
			}

			return submitWebPrompt(ec, r, webAddrID, payload.Title, payload.Format())
		})

		bind := wCfg.Bind
		if bind == "" {
			bind = "127.0.0.1:8080"
		}

		go func(srv *echo.Echo, addr string) {
			if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				c.app.Logger().Error("failed to start web server", "error", err)
			}
		}(e, bind)

		wg.Add(1)

		go func(srv *echo.Echo) {
			defer wg.Done()

			<-ctx.Done()

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				c.app.Logger().Error("failed to stop web server", "error", err)
			}
		}(e)
	}

	return nil
}

func (c *cli) startMailRouters(ctx context.Context, r *router.Router, wg *sync.WaitGroup) error {
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

		wg.Add(1)

		go func() {
			defer wg.Done()

			c.runMailLoop(ctx, r, mc, mailAddrID, instanceName)
		}()
	}

	return nil
}

func (c *cli) runMailLoop(ctx context.Context, r *router.Router, cfg mail.Config, addrID, instName string) {
	msgChan := make(chan mail.Message, 10)
	defer close(msgChan)

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

	for {
		c.app.Logger().Info("Starting Mail idle connection...", "instance", instName)

		if err := mail.ConnectAndIdle(ctx, cfg, msgChan, c.app.Logger()); err != nil {
			if ctx.Err() != nil {
				return
			}

			c.app.Logger().Error("Mail connection failed, retrying later", "instance", instName, "error", err, "retry_in", mailRetryDelay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(mailRetryDelay):
		}
	}
}

func (c *cli) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	c.app.Logger().Info("Shutting down router...")
}

func submitWebPrompt(ec echo.Context, r *router.Router, webAddrID, subject, content string) error {
	respChan := make(chan string, 1)
	replyInstance := fmt.Sprintf("web-%d", time.Now().UnixNano())
	replyAddr := replyInstance + "@web"

	err := r.RegisterAddress(router.Address{
		ID:           replyAddr,
		InstanceName: replyInstance,
		System:       "web",
		Description:  "Web Temporary Reply Address",
		SendFunc: func(ctx context.Context, msg router.Message) error {
			respChan <- msg.Content
			return nil
		},
	})
	if err != nil {
		return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer r.UnregisterAddress(replyAddr)

	err = r.SubmitMessage(ec.Request().Context(), webAddrID, router.Message{
		Metadata: router.Metadata{
			From:    webAddrID,
			To:      []string{"ai"},
			Subject: subject,
			Extra: map[string]any{
				"reply_to": replyAddr,
			},
		},
		Content: content,
	})
	if err != nil {
		return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	select {
	case reply := <-respChan:
		return ec.String(http.StatusOK, reply)
	case <-time.After(60 * time.Second):
		return ec.JSON(http.StatusGatewayTimeout, map[string]string{"error": "timed out waiting for AI response"})
	}
}
