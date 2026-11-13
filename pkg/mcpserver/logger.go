package mcpserver

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// NewLogger creates the JSON logger used by the MCP servers.
func NewLogger(appName string) *slog.Logger {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	if os.Getenv("DEBUG") != "" {
		lvl.Set(slog.LevelDebug)
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})).With("app", appName)
}

// SignalContext returns a context canceled on SIGINT or SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
