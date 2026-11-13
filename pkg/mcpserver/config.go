package mcpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

// ConfigOptions describes how an MCP server loads its configuration.
type ConfigOptions struct {
	Name      string
	EnvPrefix string
	Defaults  map[string]any
	Target    any
	Logger    *slog.Logger
}

// LoadConfig reads the YAML config file and applies environment overrides.
func LoadConfig(opts ConfigOptions) error {
	viper.Reset()

	viper.SetConfigName(opts.Name)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/spark-mcp")

	for key, value := range opts.Defaults {
		viper.SetDefault(key, value)
	}

	viper.SetEnvPrefix(opts.EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("error reading config file: %w", err)
		}

		if opts.Logger != nil {
			opts.Logger.Info("No config file found, using defaults and environment variables")
		}
	}

	if err := viper.Unmarshal(opts.Target); err != nil {
		return fmt.Errorf("unable to decode into struct: %w", err)
	}

	return nil
}
