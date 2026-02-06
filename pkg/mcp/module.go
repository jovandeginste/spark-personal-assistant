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
}

type BaseModule struct {
	logger *slog.Logger
	config any
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
