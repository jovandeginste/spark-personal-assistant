package app

import (
	"log/slog"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type App struct {
	ConfigFile string
	Config     Config

	logger *slog.Logger

	mcpClients map[string]*mcp.ClientSession
	mcpCleanup []func()
	mcpTools   map[string][]ai.Tool
}

func NewApp() *App {
	a := &App{
		mcpClients: make(map[string]*mcp.ClientSession),
		mcpTools:   make(map[string][]ai.Tool),
	}

	return a
}

func (a *App) Logger() *slog.Logger {
	return a.logger
}

func (a *App) Initialize() error {
	a.initializeLogger()

	if err := a.ReadConfig(); err != nil {
		return err
	}

	if err := a.InitializeMCP(); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeLogger() {
	a.logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
