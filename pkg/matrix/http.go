package matrix

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/web"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"
)

func (mc *MatrixConfig) ServeHTTP(ctx context.Context, instanceName string) {
	// Echo instance
	e := echo.New()

	// Use slog adapter for Echo middleware and internal logging
	e.Use(slogecho.New(mc.App.Logger()))

	// Middleware
	e.Use(middleware.Recover())

	webConfig := mc.App.Config.Webserver[instanceName]
	e.Use(web.TokenAuth(webConfig.Token))

	e.HideBanner = true
	e.HidePort = true

	// Routes
	e.GET("/prompt", mc.prompt)
	e.POST("/prompt", mc.postPrompt)
	e.GET("/persona", mc.switchPersona)
	e.GET("/update", mc.update)

	bind := webConfig.Bind
	if bind == "" {
		bind = "127.0.0.1:8080"
	}

	go func() {
		// Start server
		if err := e.Start(bind); err != nil && !errors.Is(err, http.ErrServerClosed) {
			mc.App.Logger().Error("failed to start server", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.Shutdown(shutdownCtx); err != nil {
			mc.App.Logger().Error("failed to stop server", "error", err)
		}
	}()

	mc.App.Logger().Info("HTTP server started", "bind", bind, "instance", instanceName)
}

// Handler
func (mc *MatrixConfig) prompt(c echo.Context) error {
	prompt := c.QueryParam("prompt")
	mc.sendNotice(mc.DefaultRoomID(), "", "Parsing prompt: "+prompt)

	response, err := mc.calculateInputResponse(mc.DefaultRoomID(), "", "web", prompt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.String(http.StatusOK, response)
}

func (mc *MatrixConfig) postPrompt(c echo.Context) error {
	var payload web.PromptPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON format"})
	}

	if payload.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Title is required"})
	}

	mc.sendNotice(mc.DefaultRoomID(), "", "Parsing prompt from "+payload.Source+": "+payload.Title)

	response, err := mc.calculateInputResponse(mc.DefaultRoomID(), "", "web", payload.Format())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	return c.String(http.StatusOK, response)
}

func (mc *MatrixConfig) switchPersona(c echo.Context) error {
	persona := c.QueryParam("persona")
	if persona == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "persona query parameter is required"})
	}

	mc.sendNotice(mc.DefaultRoomID(), "", "Switching to persona: "+persona)

	response, err := mc.performCommandSwitchPersona(persona)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.String(http.StatusOK, response)
}

func (mc *MatrixConfig) update(c echo.Context) error {
	results := mc.App.UpdateMCPServers(c.Request().Context())

	success := true
	for _, msg := range results {
		if msg != "Success" {
			success = false
			break
		}
	}

	if !success {
		return c.JSON(http.StatusInternalServerError, results)
	}
	return c.JSON(http.StatusOK, results)
}
