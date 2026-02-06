package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/caching"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/diary"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/googlecontacts"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/ical"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/kitchenowl"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/simplemarkdown"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/vcf"
	"github.com/jovandeginste/spark-personal-assistant/pkg/mcp/weather"
	"github.com/jovandeginste/workout-tracker/v2/pkg/geocoder"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
)

type Config struct {
	KitchenOwl     kitchenowl.Config     `mapstructure:"kitchenowl"`
	Weather        weather.Config        `mapstructure:"weather"`
	ICal           ical.Config           `mapstructure:"ical"`
	VCF            vcf.Config            `mapstructure:"vcf"`
	SimpleMarkdown simplemarkdown.Config `mapstructure:"simplemarkdown"`
	Diary          diary.Config          `mapstructure:"diary"`
	GoogleContacts googlecontacts.Config `mapstructure:"googlecontacts"`
	Port           string                `mapstructure:"port"`
}

func main() {
	// Initialize structured logging with JSON handler
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	geocoder.SetClient(logger, "Spark")

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
		"vcf_enabled", config.VCF.Path != "",
		"simplemarkdown_enabled", config.SimpleMarkdown.Path != "",
		"diary_enabled", config.Diary.Path != "",
		"googlecontacts_enabled", config.GoogleContacts.TokenFile != "",
		"port", config.Port,
	)

	// Initialize caching service
	cacheService, err := caching.NewService("./tmp/cache", 6*time.Hour)
	if err != nil {
		slog.Error("failed to initialize caching service", "error", err)
		os.Exit(1)
	}

	// Create SSE handler
	sseHandler := sdk.NewSSEHandler(func(r *http.Request) *sdk.Server {
		server := sdk.NewServer(&sdk.Implementation{
			Name:    "mcp-personal-data",
			Version: "1.0.0",
		}, &sdk.ServerOptions{
			Logger: logger,
		})

		modules := []mcp.Module{}

		// Weather
		weatherModule := &weather.Module{}
		weatherModule.SetConfig(config.Weather)
		modules = append(modules, weatherModule)

		// KitchenOwl
		if config.KitchenOwl.Token != "" {
			kitchenOwlModule := &kitchenowl.Module{Cache: cacheService}
			kitchenOwlModule.SetConfig(config.KitchenOwl)
			modules = append(modules, kitchenOwlModule)
		} else {
			slog.Info("KitchenOwl tool disabled (no token provided)")
		}

		// ICal
		icalModule := &ical.Module{Cache: cacheService}
		icalModule.SetConfig(config.ICal)
		modules = append(modules, icalModule)

		// VCF
		vcfModule := &vcf.Module{Cache: cacheService}
		vcfModule.SetConfig(config.VCF)
		modules = append(modules, vcfModule)

		// SimpleMarkdown
		simpleMarkdownModule := &simplemarkdown.Module{}
		simpleMarkdownModule.SetConfig(config.SimpleMarkdown)
		modules = append(modules, simpleMarkdownModule)

		// Diary
		diaryModule := &diary.Module{}
		diaryModule.SetConfig(config.Diary)
		modules = append(modules, diaryModule)

		// Google Contacts
		googleContactsModule := &googlecontacts.Module{}
		googleContactsModule.SetConfig(config.GoogleContacts)
		modules = append(modules, googleContactsModule)

		for _, module := range modules {
			module.SetLogger(logger)
			if err := module.Initialize(); err != nil {
				slog.Error("failed to initialize module", "module", fmt.Sprintf("%T", module), "error", err)
				continue
			}
			if err := module.Register(server); err != nil {
				slog.Error("failed to register module", "module", fmt.Sprintf("%T", module), "error", err)
			}
		}

		return server
	}, nil)

	// Prefetch calendars in background
	// ical.StartPrefetch(config.ICal, cacheService, logger)
	icalModule := &ical.Module{Cache: cacheService}
	icalModule.SetConfig(config.ICal)
	icalModule.SetLogger(logger)
	icalModule.StartPrefetch()

	slog.Info("Starting SSE server on " + config.Port)

	mux := http.NewServeMux()
	mux.Handle("/sse", sseHandler)
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		slog.Info("Update requested, clearing caches")
		if err := cacheService.Clear(); err != nil {
			slog.Error("Failed to clear cache", "error", err)
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
			slog.Error("Failed to recreate cache directory", "error", err)
		}

		// Trigger prefetch again
		// go ical.StartPrefetch(config.ICal, cacheService, slog.Default())
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
