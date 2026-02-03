package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/ical"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/kitchenowl"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/simplemarkdown"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/weather"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
)

type Config struct {
	KitchenOwl     kitchenowl.Config     `mapstructure:"kitchenowl"`
	Weather        weather.Config        `mapstructure:"weather"`
	ICal           ical.Config           `mapstructure:"ical"`
	SimpleMarkdown simplemarkdown.Config `mapstructure:"simplemarkdown"`
	Port           string                `mapstructure:"port"`
}

func main() {
	geocoder.SetClient(slog.Default(), "Spark")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Log loaded configuration (excluding URLs/Tokens)
	slog.Info("Configuration loaded",
		"weather_enabled", config.Weather.APIURL != "",
		"kitchenowl_enabled", config.KitchenOwl.Token != "",
		"ical_calendars", len(config.ICal.Calendars),
		"simplemarkdown_enabled", config.SimpleMarkdown.Path != "",
		"port", config.Port,
	)

	for _, cal := range config.ICal.Calendars {
		slog.Info("ICal Calendar configured", "name", cal.Name, "description", cal.Description)
	}

	// Initialize caching service
	cacheService, err := caching.NewService("./tmp/cache", 12*time.Hour)
	if err != nil {
		slog.Error("failed to initialize caching service", "error", err)
		os.Exit(1)
	}

	// Create SSE handler
	sseHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "mcp-personal-data",
			Version: "1.0.0",
		}, &mcp.ServerOptions{
			Logger: slog.Default(),
		})

		// Register tools
		logger := slog.Default()
		if err := weather.Register(server, config.Weather, logger); err != nil {
			slog.Error("failed to register weather tool", "error", err)
			return nil
		}

		if config.KitchenOwl.Token != "" {
			if err := kitchenowl.Register(server, config.KitchenOwl, cacheService, logger); err != nil {
				slog.Error("failed to register kitchenowl tool", "error", err)
				return nil
			}
		} else {
			slog.Info("KitchenOwl tool disabled (no token provided)")
		}

		if err := ical.Register(server, config.ICal, cacheService, logger); err != nil {
			slog.Error("failed to register ical tool", "error", err)
			return nil
		}

		if err := simplemarkdown.Register(server, config.SimpleMarkdown, logger); err != nil {
			slog.Error("failed to register simplemarkdown tool", "error", err)
			return nil
		}

		return server
	}, nil)

	// Prefetch calendars in background
	ical.StartPrefetch(config.ICal, cacheService, slog.Default())

	slog.Info("Starting SSE server on " + config.Port)

	mux := http.NewServeMux()
	mux.Handle("/sse", sseHandler)

	server := &http.Server{
		Addr:              config.Port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("mcp-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/spark-mcp")

	// Set defaults
	viper.SetDefault("port", ":8081")
	viper.SetDefault("weather.apiurl", "https://api.open-meteo.com/v1/forecast")

	viper.SetDefault("kitchenowl.apiurl", "https://kitchenowl.thuis.dwarfy.be/api")
	viper.SetDefault("kitchenowl.householdid", 1)

	// Environment variables override config file
	viper.SetEnvPrefix("MCP")
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
