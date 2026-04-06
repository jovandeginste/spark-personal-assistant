package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/drillster"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/twizzit"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
)

type Config struct {
	Drillster drillster.Config `mapstructure:"drillster"`
	Twizzit   twizzit.Config   `mapstructure:"twizzit"`
	Port      string           `mapstructure:"port"`
}

func main() {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	if os.Getenv("DEBUG") != "" {
		lvl.Set(slog.LevelDebug)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})).With("app", "spark-mcp-hockey")

	config, err := loadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	modules := []mcp.Module{
		drillster.New(config.Drillster, logger),
		twizzit.New(config.Twizzit, logger),
	}

	for _, module := range modules {
		if err := module.Initialize(); err != nil {
			logger.Error("failed to initialize module", "module", fmt.Sprintf("%T", module), "error", err)
			continue
		}
	}

	sseHandler := sdk.NewSSEHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{
			Name:    "spark-mcp-hockey",
			Version: "1.0.0",
		}, &sdk.ServerOptions{
			Logger: logger,
		})

		for _, module := range modules {
			if err := module.Enabled(); err != nil {
				logger.Info("Module disabled", "module", fmt.Sprintf("%T", module), "reason", err)
				continue
			}

			if err := module.Register(server); err != nil {
				logger.Error("failed to register module", "module", fmt.Sprintf("%T", module), "error", err)
				continue
			}

			logger.Info("Module registered", "module", fmt.Sprintf("%T", module))
		}

		return server
	}, nil)

	streamableHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{
			Name:    "spark-mcp-hockey",
			Version: "1.0.0",
		}, &sdk.ServerOptions{
			Logger: logger,
		})

		for _, module := range modules {
			if err := module.Enabled(); err != nil {
				logger.Info("Module disabled", "module", fmt.Sprintf("%T", module), "reason", err)
				continue
			}

			if err := module.Register(server); err != nil {
				logger.Error("failed to register module", "module", fmt.Sprintf("%T", module), "error", err)
				continue
			}

			logger.Info("Module registered", "module", fmt.Sprintf("%T", module))
		}

		return server
	}, nil)

	logger.Info("Starting server on " + config.Port)

	mux := http.NewServeMux()
	mux.Handle("/sse", sseHandler)
	mux.Handle("/message", sseHandler)
	mux.Handle("/", streamableHandler)

	server := &http.Server{
		Addr:              config.Port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("mcp-hockey")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/spark-mcp")

	viper.SetDefault("port", ":8082")

	viper.SetEnvPrefix("HOCKEY")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		slog.Info("No config file found, using defaults and environment variables")
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &config, nil
}
