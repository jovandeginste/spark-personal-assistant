package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/googlecontacts"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/ical"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/jsonreader"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/recycleapp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/reminders"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/vcf"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/weather"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/webfetcher"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/websearch"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcpserver"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
)

type Config struct {
	Weather        weather.Config        `mapstructure:"weather"`
	ICal           ical.Config           `mapstructure:"ical"`
	VCF            vcf.Config            `mapstructure:"vcf"`
	GoogleContacts googlecontacts.Config `mapstructure:"googlecontacts"`
	JSONReader     jsonreader.Config     `mapstructure:"jsonreader"`
	WebFetcher     webfetcher.Config     `mapstructure:"webfetcher"`
	WebSearch      websearch.Config      `mapstructure:"websearch"`
	Reminders      reminders.Config      `mapstructure:"reminders"`
	RecycleApp     recycleapp.Config     `mapstructure:"recycleapp"`
	Port           string                `mapstructure:"port"`
}

func main() {
	logger := mcpserver.NewLogger("spark-mcp-assistant")

	geocoder.SetClient(logger, "Spark")

	config, err := loadConfig(logger)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	cacheService, err := caching.NewService("./tmp/cache", 6*time.Hour)
	if err != nil {
		logger.Error("failed to initialize caching service", "error", err)
		os.Exit(1)
	}

	modules, icalModule := allModules(config, logger, cacheService)
	server := mcpserver.New("mcp-personal-data", "1.0.0", modules, logger)

	ctx, stop := mcpserver.SignalContext()

	logger.Info("Starting server on " + config.Port)

	err = server.Run(ctx, config.Port, func(mux *http.ServeMux) {
		mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			logger.Info("Update requested, clearing caches")

			if err := cacheService.Clear(); err != nil {
				logger.Error("Failed to clear cache", "error", err)
				http.Error(w, "Failed to clear cache", http.StatusInternalServerError)

				return
			}

			icalModule.StartPrefetch()

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Cache cleared and updates triggered"))
		})
	})

	stop()

	if err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func loadConfig(logger *slog.Logger) (*Config, error) {
	config := &Config{}

	err := mcpserver.LoadConfig(mcpserver.ConfigOptions{
		Name:      "mcp-config",
		EnvPrefix: "MCP",
		Defaults: map[string]any{
			"port":           ":8081",
			"weather.apiurl": "https://api.open-meteo.com/v1/forecast",
		},
		Target: config,
		Logger: logger,
	})

	return config, err
}

func allModules(config *Config, logger *slog.Logger, cacheService *caching.Service) ([]mcp.Module, *ical.Module) {
	icalModule := ical.New(config.ICal, cacheService, logger)

	modules := []mcp.Module{
		weather.New(config.Weather, logger),
		icalModule,
		vcf.New(config.VCF, logger),
		googlecontacts.New(config.GoogleContacts, logger),
		jsonreader.New(config.JSONReader, cacheService, logger),
		webfetcher.New(config.WebFetcher, logger),
		websearch.New(config.WebSearch, logger),
		reminders.New(config.Reminders, logger),
		recycleapp.New(config.RecycleApp, logger),
	}

	return modules, icalModule
}
