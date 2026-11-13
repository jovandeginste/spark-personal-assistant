package main

import (
	"log/slog"
	"os"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/drillster"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/twizzit"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcpserver"
)

type Config struct {
	Drillster drillster.Config `mapstructure:"drillster"`
	Twizzit   twizzit.Config   `mapstructure:"twizzit"`
	Port      string           `mapstructure:"port"`
}

func main() {
	logger := mcpserver.NewLogger("spark-mcp-hockey")

	config, err := loadConfig(logger)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	modules := []mcp.Module{
		drillster.New(config.Drillster, logger),
		twizzit.New(config.Twizzit, logger),
	}

	server := mcpserver.New("spark-mcp-hockey", "1.0.0", modules, logger)

	ctx, stop := mcpserver.SignalContext()

	logger.Info("Starting server on " + config.Port)

	err = server.Run(ctx, config.Port, nil)

	stop()

	if err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig(logger *slog.Logger) (*Config, error) {
	config := &Config{}

	err := mcpserver.LoadConfig(mcpserver.ConfigOptions{
		Name:      "mcp-hockey",
		EnvPrefix: "HOCKEY",
		Defaults: map[string]any{
			"port": ":8082",
		},
		Target: config,
		Logger: logger,
	})

	return config, err
}
