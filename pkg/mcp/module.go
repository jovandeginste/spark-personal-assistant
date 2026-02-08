package mcp

import (
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Module interface {
	SetLogger(logger *slog.Logger)
	Logger() *slog.Logger
	SetConfig(config any)
	Config() any
	Initialize() error
	Register(server *mcp.Server) error
	Enabled() error
}

type BaseModule struct {
	logger *slog.Logger
	config any
}

func NewBaseModule(config any, logger *slog.Logger) BaseModule {
	return BaseModule{
		config: config,
		logger: logger,
	}
}

func (b *BaseModule) SetLogger(logger *slog.Logger) {
	b.logger = logger
}

func (b *BaseModule) Logger() *slog.Logger {
	return b.logger
}

func (b *BaseModule) SetConfig(config any) {
	b.config = config
}

func (b *BaseModule) Config() any {
	return b.config
}

func (b *BaseModule) Initialize() error {
	return nil
}

func (b *BaseModule) Register(server *mcp.Server) error {
	return nil
}

func (b *BaseModule) Enabled() error {
	return nil
}
