package matrix

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (mc *MatrixConfig) ServeHTTP() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
			)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	e.HideBanner = true
	e.HidePort = true

	// Routes
	e.GET("/prompt", mc.prompt)
	e.GET("/persona", mc.switchPersona)
	e.GET("/update", mc.update)

	go func() {
		// Start server
		if err := e.Start(mc.App.Config.Webserver.Bind); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to start server", "error", err)
		}
	}()

	mc.App.Logger().Info("HTTP server started", "bind", mc.App.Config.Webserver.Bind)
}

// Handler
func (mc *MatrixConfig) prompt(c echo.Context) error {
	prompt := c.QueryParam("prompt")
	mc.sendNotice(mc.DefaultRoomID(), "Parsing prompt: "+prompt)

	if err := mc.sendResponse(mc.DefaultRoomID(), "web", prompt); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.String(http.StatusOK, "Prompt sent")
}

func (mc *MatrixConfig) switchPersona(c echo.Context) error {
	persona := c.QueryParam("persona")
	mc.sendNotice(mc.DefaultRoomID(), "Switching to persona: "+persona)

	if err := mc.sendResponse(mc.DefaultRoomID(), "web", "switch "+persona); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.String(http.StatusOK, "Persona switched")
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
