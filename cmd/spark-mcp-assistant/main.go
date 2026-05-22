package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/googlecontacts"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/ical"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/jsonreader"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/projects"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/reminders"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/vcf"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/weather"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/webfetcher"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/websearch"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
)

type Config struct {
	Weather        weather.Config        `mapstructure:"weather"`
	ICal           ical.Config           `mapstructure:"ical"`
	VCF            vcf.Config            `mapstructure:"vcf"`
	Projects       projects.Config       `mapstructure:"projects"`
	GoogleContacts googlecontacts.Config `mapstructure:"googlecontacts"`
	JSONReader     jsonreader.Config     `mapstructure:"jsonreader"`
	WebFetcher     webfetcher.Config     `mapstructure:"webfetcher"`
	WebSearch      websearch.Config      `mapstructure:"websearch"`
	Reminders      reminders.Config      `mapstructure:"reminders"`
	Port           string                `mapstructure:"port"`
}

func main() {
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	if os.Getenv("DEBUG") != "" {
		lvl.Set(slog.LevelDebug)
	}

	// Initialize structured logging with JSON handler
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})).With("app", "spark-mcp-assistant")

	geocoder.SetClient(logger, "Spark")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize caching service
	cacheService, err := caching.NewService("./tmp/cache", 6*time.Hour)
	if err != nil {
		logger.Error("failed to initialize caching service", "error", err)
		os.Exit(1)
	}

	modules := allModules(config, logger, cacheService)

	for _, module := range modules {
		if err := module.Initialize(); err != nil {
			logger.Error("failed to initialize module", "module", fmt.Sprintf("%T", module), "error", err)
			continue
		}
	}

	// Create Streamable handler
	streamableHandler := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{
			Name:    "mcp-personal-data",
			Version: "1.0.0",
		}, &sdk.ServerOptions{
			Logger: logger,
			Capabilities: &sdk.ServerCapabilities{
				Tools: &sdk.ToolCapabilities{ListChanged: false},
			},
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
	}, &sdk.StreamableHTTPOptions{
		Stateless: false,
	})

	logger.Info("Starting server on " + config.Port)

	mux := http.NewServeMux()
	mux.Handle("/", streamableHandler)
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

		// Re-initialize cache service logic if needed (e.g. recreating directory)
		// Clear() removes the directory, so we should ensure it exists for next use
		// Actually cacheService.Clear() -> removeAll, so next usage might fail if directory expected?
		// NewService creates it. Let's rely on cacheService methods to handle recreation or just recreate the directory here.
		// Looking at caching.go, NewService does MkdirAll.
		// Let's just manually recreate the directory to be safe after RemoveAll
		if err := os.MkdirAll("./tmp/cache", 0o755); err != nil {
			logger.Error("Failed to recreate cache directory", "error", err)
		}

		// Trigger prefetch again
		icalModule := &ical.Module{Cache: cacheService}
		icalModule.SetConfig(config.ICal)
		icalModule.SetLogger(logger)
		icalModule.StartPrefetch()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Cache cleared and updates triggered"))
	})

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
	viper.SetConfigName("mcp-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/spark-mcp")

	// Set defaults
	viper.SetDefault("port", ":8081")
	viper.SetDefault("weather.apiurl", "https://api.open-meteo.com/v1/forecast")

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

func allModules(config *Config, logger *slog.Logger, cacheService *caching.Service) []mcp.Module {
	modules := []mcp.Module{
		weather.New(config.Weather, logger),
		ical.New(config.ICal, cacheService, logger),
		vcf.New(config.VCF, cacheService, logger),
		projects.New(config.Projects, logger),
		googlecontacts.New(config.GoogleContacts, logger),
		jsonreader.New(config.JSONReader, logger),
		webfetcher.New(config.WebFetcher, logger),
		websearch.New(config.WebSearch, logger),
		reminders.New(config.Reminders, logger),
	}

	return modules
}
