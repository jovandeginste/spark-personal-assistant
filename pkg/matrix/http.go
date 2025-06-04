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
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.HideBanner = true
	e.HidePort = true

	// Routes
	e.GET("/prompt", mc.prompt)
	e.GET("/update", mc.updateSources)

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
	if err := mc.sendResponse(mc.DefaultRoomID(), "web", prompt); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.String(http.StatusOK, "Prompt sent")
}

func (mc *MatrixConfig) updateSources(c echo.Context) error {
	if err := mc.App.UpdateSources(); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.String(http.StatusOK, "Sources updated")
}
