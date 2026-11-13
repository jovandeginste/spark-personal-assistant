package app

import (
	"log/slog"
	"os"
	"sync"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type App struct {
	ConfigFile string
	Config     Config

	logger *slog.Logger

	mcpMu      sync.Mutex
	mcpClients map[string]*mcp.ClientSession
	mcpCleanup map[string]func()
	mcpTools   map[string][]ai.Tool
}

func NewApp() *App {
	a := &App{
		mcpClients: make(map[string]*mcp.ClientSession),
		mcpCleanup: make(map[string]func()),
		mcpTools:   make(map[string][]ai.Tool),
	}

	return a
}

// Close releases all resources held by the app.
func (a *App) Close() {
	a.mcpMu.Lock()
	cleanup := a.mcpCleanup
	a.mcpCleanup = make(map[string]func())
	a.mcpClients = make(map[string]*mcp.ClientSession)
	a.mcpTools = make(map[string][]ai.Tool)
	a.mcpMu.Unlock()

	for _, fn := range cleanup {
		fn()
	}
}

func (a *App) Logger() *slog.Logger {
	return a.logger
}

func (a *App) Initialize() error {
	a.initializeLogger()

	if err := a.ReadConfig(); err != nil {
		return err
	}

	return nil
}

func (a *App) initializeLogger() {
	a.logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
