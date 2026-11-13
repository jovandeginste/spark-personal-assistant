package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	sparkmcp "github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const shutdownTimeout = 5 * time.Second

// Server wraps an MCP streamable HTTP server and its modules.
type Server struct {
	name    string
	version string
	modules []sparkmcp.Module
	logger  *slog.Logger
}

// New creates a server and initializes all modules. Initialization failures are
// logged and do not prevent the server from starting.
func New(name, version string, modules []sparkmcp.Module, logger *slog.Logger) *Server {
	enabled := make([]sparkmcp.Module, 0, len(modules))

	for _, module := range modules {
		if err := module.Enabled(); err != nil {
			logger.Info("Module disabled", "module", fmt.Sprintf("%T", module), "reason", err)

			continue
		}

		if err := module.Initialize(); err != nil {
			logger.Error("failed to initialize module", "module", fmt.Sprintf("%T", module), "error", err)
		}

		enabled = append(enabled, module)
	}

	return &Server{
		name:    name,
		version: version,
		modules: enabled,
		logger:  logger,
	}
}

// Run listens on addr and shuts down gracefully when ctx is canceled.
func (s *Server) Run(ctx context.Context, addr string, extraRoutes func(*http.ServeMux)) error {
	defer s.closeModules()

	mux := http.NewServeMux()
	mux.Handle("/", s.handler())

	if extraRoutes != nil {
		extraRoutes(mux)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	}
}

func (s *Server) closeModules() {
	for _, module := range s.modules {
		if err := module.Close(); err != nil {
			s.logger.Error("failed to close module", "module", fmt.Sprintf("%T", module), "error", err)
		}
	}
}

func (s *Server) handler() http.Handler {
	return sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{
			Name:    s.name,
			Version: s.version,
		}, &sdk.ServerOptions{
			Logger: s.logger,
			Capabilities: &sdk.ServerCapabilities{
				Tools: &sdk.ToolCapabilities{ListChanged: false},
			},
		})

		for _, module := range s.modules {
			if err := module.Register(server); err != nil {
				s.logger.Error("failed to register module", "module", fmt.Sprintf("%T", module), "error", err)

				continue
			}

			s.logger.Info("Module registered", "module", fmt.Sprintf("%T", module))
		}

		return server
	}, &sdk.StreamableHTTPOptions{
		Stateless: true,
	})
}
